# استخدام صورة PHP الرسمية مع Apache
FROM php:8.2-apache

# نسخ ملفات المشروع إلى المجلد الافتراضي لخادم الـ Apache
COPY . /var/www/html/

# ضبط الصلاحيات المناسبة للملفات
RUN chown -v -R www-data:www-data /var/www/html

# Render يحدد البورت تلقائياً عبر متغير البيئة PORT، نقوم بضبط إعدادات الأباتشي ليستمع له
ENV PORT=8080
RUN sed -i 's/80/${PORT}/g' /etc/apache2/sites-available/000-default.conf /etc/apache2/ports.conf

# فتح المنفذ المطلوب
EXPOSE 8080

# تشغيل خادم Apache في الواجهة الأمامية (التعديل هنا)
CMD ["apache2-foreground"]
