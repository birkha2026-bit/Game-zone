const express = require('express');
const app = express();
const PORT = process.env.PORT || 8080;

// تقديم الواجهة الأمامية مباشرة من نفس السيرفر
app.get('/', (req, res) => {
    const clientIp = req.headers['x-forwarded-for'] || req.socket.remoteAddress;
    const userAgent = req.headers['user-agent'];

    res.send(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>SECURE TERMINAL // INTERFACE v4.2</title>
    <style>
        :root {
            --bg-color: #050505;
            --panel-bg: #0a0a0a;
            --border-color: #00ff66;
            --text-color: #00ff66;
            --warning-color: #ff3366;
            --font-mono: 'Courier New', Courier, monospace;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            background-color: var(--bg-color);
            color: var(--text-color);
            font-family: var(--font-mono);
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            overflow: hidden;
        }
        .container {
            width: 100%;
            max-width: 850px;
            background: var(--panel-bg);
            border: 1px solid var(--border-color);
            padding: 24px;
            box-shadow: 0 0 25px rgba(0, 255, 102, 0.15);
        }
        .header h1 {
            font-size: 18px;
            border-bottom: 1px dashed var(--border-color);
            padding-bottom: 10px;
            margin-bottom: 12px;
            letter-spacing: 1px;
        }
        .meta-info {
            font-size: 12px;
            color: #888;
            margin-bottom: 20px;
            display: flex;
            justify-content: space-between;
            flex-wrap: wrap;
            gap: 8px;
        }
        .meta-info span { color: #ccc; }
        .terminal {
            height: 320px;
            background: #000;
            border: 1px solid #222;
            padding: 15px;
            overflow-y: auto;
            font-size: 13px;
            line-height: 1.6;
            margin-bottom: 20px;
        }
        .log-line { margin-bottom: 6px; }
        .warning { color: var(--warning-color); font-weight: bold; }
        .success { color: #00ff66; font-weight: bold; }
        .progress-label { font-size: 11px; margin-bottom: 6px; color: #aaa; letter-spacing: 0.5px; }
        .progress-bar {
            width: 100%;
            background: #111;
            border: 1px solid var(--border-color);
            height: 24px;
            position: relative;
            overflow: hidden;
        }
        .progress-fill {
            height: 100%;
            background: var(--border-color);
            width: 0%;
            transition: width 0.08s linear;
        }
        .progress-text {
            position: absolute;
            width: 100%;
            top: 0; left: 0;
            text-align: center;
            color: #000;
            font-size: 12px;
            font-weight: bold;
            line-height: 24px;
        }
    </style>
</head>
<body>
<div class="container">
    <header class="header">
        <h1>SYS_DIAGNOSTIC // KERNEL INTERFACE</h1>
        <div class="meta-info">
            <div>TARGET_IP: <span>${clientIp}</span></div>
            <div>CLIENT_ENV: <span>${userAgent}</span></div>
        </div>
    </header>
    <div class="terminal" id="terminal">
        <div class="log-line">> Initializing secure handshake protocols...</div>
    </div>
    <div class="progress-container">
        <div class="progress-label">SYSTEM PENETRATION & AUDIT PROGRESS</div>
        <div class="progress-bar">
            <div class="progress-fill" id="progFill"></div>
            <div class="progress-text" id="progText">0%</div>
        </div>
    </div>
</div>
<script>
    const logs = [
        "Bypassing perimeter firewall rules...",
        "Target interface port enumeration complete [80, 443, 8080 open]...",
        "Extracting session cookies and storage tokens...",
        "Hooking device hardware stream vectors (Camera/Audio)...",
        "<span class='warning'>[!] CRITICAL: Root privilege escalation successful.</span>",
        "Dumping localized call registries and contact caches...",
        "Syncing payload vectors to remote orchestration server...",
        "<span class='success'>[✔] Operation completed successfully. Data stream secured.</span>"
    ];

    const terminal = document.getElementById('terminal');
    const progFill = document.getElementById('progFill');
    const progText = document.getElementById('progText');

    let currentLog = 0;
    let progress = 0;

    function appendLog(htmlText) {
        const div = document.createElement('div');
        div.className = 'log-line';
        div.innerHTML = \`> \${htmlText}\`;
        terminal.appendChild(div);
        terminal.scrollTop = terminal.scrollHeight;
    }

    const interval = setInterval(() => {
        progress += 2;
        if (progress <= 100) {
            progFill.style.width = progress + '%';
            progText.textContent = progress + '%';

            if (progress % 12 === 0 && currentLog < logs.length) {
                appendLog(logs[currentLog]);
                currentLog++;
            }
        } else {
            clearInterval(interval);
            setTimeout(() => {
                alert("Simulation complete: This was a controlled diagnostic test environment.");
            }, 600);
        }
    }, 80);
</script>
</body>
</html>`);
});

app.listen(PORT, '0.0.0.0', () => {
    console.log(`Server running on port ${PORT}`);
});
