<?php
// صفحة مزحة لمحاكاة الاختراق
$target_ip = $_SERVER['REMOTE_ADDR'];
$user_agent = $_SERVER['HTTP_USER_AGENT'];
?>
<!DOCTYPE html>
<html lang="ar" dir="rtl">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>System Diagnostic & Terminal</title>
    <style>
        body {
            background-color: #0d0208;
            color: #00ff66;
            font-family: 'Courier New', Courier, monospace;
            margin: 0;
            padding: 20px;
            overflow-x: hidden;
        }
        .container {
            max-width: 800px;
            margin: 0 auto;
            border: 1px solid #00ff66;
            padding: 20px;
            box-shadow: 0 0 15px rgba(0, 255, 102, 0.3);
            background-color: rgba(0, 0, 0, 0.85);
        }
        h1 {
            text-align: center;
            border-bottom: 1px dashed #00ff66;
            padding-bottom: 10px;
            font-size: 24px;
            text-shadow: 0 0 5px #00ff66;
        }
        .terminal {
            height: 300px;
            overflow-y: auto;
            background: #000;
            border: 1px solid #333;
            padding: 10px;
            margin-bottom: 20px;
            font-size: 14px;
            line-height: 1.5;
        }
        .log-line {
            margin: 5px 0;
        }
        .warning {
            color: #ff0055;
            font-weight: bold;
        }
        .success {
            color: #00ff66;
        }
        .progress-bar {
            width: 100%;
            background: #111;
            border: 1px solid #00ff66;
            height: 25px;
            margin-top: 15px;
            position: relative;
        }
        .progress-fill {
            height: 100%;
            background: #00ff66;
            width: 0%;
            transition: width 0.1s linear;
        }
        .progress-text {
            position: absolute;
            width: 100%;
            text-align: center;
            color: #000;
            font-weight: bold;
            line-height: 25px;
        }
    </style>
</head>
<body>

<div class="container">
    <h1>⚠️ SECURE TERMINAL v4.0 - DEVICE INTERFACE ⚠️</h1>
    
    <div style="margin-bottom: 15px; font-size: 13px; color: #888;">
        <div>معرّف الجهاز (IP): <?php echo htmlspecialchars($target_ip); ?></div>
        <div>النظام المستهدف: <?php echo htmlspecialchars($user_agent); ?></div>
    </div>

    <div class="terminal" id="terminal">
        <div class="log-line">> تهيئة الاتصال بالسيرفر الآمن...</div>
        <div class="log-line">> تم تحديد عنوان الجهاز بنجاح.</div>
    </div>

    <div>جاري فحص واختراق الحماية...</div>
    <div class="progress-bar">
        <div class="progress-text" id="progText">0%</div>
        <div class="progress-fill" id="progFill"></div>
    </div>
</div>

<script>
    const logs = [
        "جاري تجاوز جدار الحماية (Firewall)...",
        "تم العثور على منفذ مفتوح: Port 8080",
        "جاري سحب ملفات تعريف الارتباط (Cookies)...",
        "قراءة بيانات الكاميرا الأمامية والخلفية...",
        "<span class='warning'>[!] تحذير: تم الوصول إلى صلاحيات الجذر (Root Access)...</span>",
        "جاري تفريغ سجلات المكالمات والرسائل...",
        "تحميل الصور وجهات الاتصال إلى الخادم البعيد...",
        "<span class='success'>[✔] تمت العملية بنجاح! تم استخراج كافة البيانات.</span>"
    ];

    const terminal = document.getElementById('terminal');
    const progFill = document.getElementById('progFill');
    const progText = document.getElementById('progText');

    let currentLog = 0;
    let progress = 0;

    function addLog(text) {
        const div = document.createElement('div');
        div.className = 'log-line';
        div.innerHTML = text;
        terminal.appendChild(div);
        terminal.scrollTop = terminal.scrollHeight;
    }

    const interval = setInterval(() => {
        progress += 2;
        if (progress <= 100) {
            progFill.style.width = progress + '%';
            progText.textContent = progress + '%';

            // إضافة رسالة جديدة كل نسبة معينة
            if (progress % 12 === 0 && currentLog < logs.length) {
                addLog("> " + logs[currentLog]);
                currentLog++;
            }
        } else {
            clearInterval(interval);
            setTimeout(() => {
                alert("مفاجأة! هذه مجرد مزحة برمجية 😜");
            }, 500);
        }
    }, 100);
</script>

</body>
</html>
