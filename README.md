# **certificate-validate**

Validate some info in SSL/TLS Certificates

## **prerequisites**

* Docker
* Internet Access

## **build**

```shell
docker build --no-cache --rm -t <NAME_OF_IMAGE> -f ./Dockerfile .
```

## **run**

| **variable** | **description** |
| ------------- | --------------- |
| CERTIFICATE_URL | URL of the certificate to validate |
| CERTIFICATE_PORT | Port of the certificate to validate |
| CERTIFICATE_TIME_TO_WAIT | Time to wait for the certificate to be validated, is optional, if not set, it will be set to **86400** |

```shell
docker run -d --name certificate_validate_google \
-e CERTIFICATE_URL=google.com \
-e CERTIFICATE_PORT=443 \
<NAME_OF_IMAGE>
```

```shell
docker ps

CONTAINER ID   IMAGE                                 COMMAND                CREATED          STATUS          PORTS     NAMES
e3b9598147db   fabianosanflor/certificate:validate   "/app/entrypoint.sh"   29 minutes ago   Up 29 minutes             certificate_validate_google
```

```shell
docker exec -it <CONTAINER NAME> cat /app/certificate.log

Ex. docker exec -it certificate_validate_google cat /app/certificate.log

{
     "commonName": "*.google.com",
     "SAN": "['*.google.com', '*.appengine.google.com', '*.bdn.dev', '*.cloud.google.com', '*.crowdsource.google.com', '*.datacompute.google.com', '*.google.ca', '*.google.cl', '*.google.co.in', '*.google.co.jp', '*.google.co.uk', '*.google.com.ar', '*.google.com.au', '*.google.com.br', '*.google.com.co', '*.google.com.mx', '*.google.com.tr', '*.google.com.vn', '*.google.de', '*.google.es', '*.google.fr', '*.google.hu', '*.google.it', '*.google.nl', '*.google.pl', '*.google.pt', '*.googleadapis.com', '*.googleapis.cn', '*.googlevideo.com', '*.gstatic.cn', '*.gstatic-cn.com', '*.gstaticcnapps.cn', 'googlecnapps.cn', '*.googlecnapps.cn', 'googleapps-cn.com', '*.googleapps-cn.com', 'gkecnapps.cn', '*.gkecnapps.cn', 'googledownloads.cn', '*.googledownloads.cn', 'recaptcha.net.cn', '*.recaptcha.net.cn', 'widevine.cn', '*.widevine.cn', 'ampproject.org.cn', '*.ampproject.org.cn', 'ampproject.net.cn', '*.ampproject.net.cn', 'google-analytics-cn.com', '*.google-analytics-cn.com', 'googleadservices-cn.com', '*.googleadservices-cn.com', 'googlevads-cn.com', '*.googlevads-cn.com', 'googleapis-cn.com', '*.googleapis-cn.com', 'googleoptimize-cn.com', '*.googleoptimize-cn.com', 'doubleclick-cn.net', '*.doubleclick-cn.net', '*.fls.doubleclick-cn.net', '*.g.doubleclick-cn.net', 'doubleclick.cn', '*.doubleclick.cn', '*.fls.doubleclick.cn', '*.g.doubleclick.cn', 'dartsearch-cn.net', '*.dartsearch-cn.net', 'googletraveladservices-cn.com', '*.googletraveladservices-cn.com', 'googletagservices-cn.com', '*.googletagservices-cn.com', 'googletagmanager-cn.com', '*.googletagmanager-cn.com', 'googlesyndication-cn.com', '*.googlesyndication-cn.com', '*.safeframe.googlesyndication-cn.com', 'app-measurement-cn.com', '*.app-measurement-cn.com', 'gvt1-cn.com', '*.gvt1-cn.com', 'gvt2-cn.com', '*.gvt2-cn.com', '2mdn-cn.net', '*.2mdn-cn.net', 'googleflights-cn.net', '*.googleflights-cn.net', 'admob-cn.com', '*.admob-cn.com', '*.gstatic.com', '*.metric.gstatic.com', '*.gvt1.com', '*.gcpcdn.gvt1.com', '*.gvt2.com', '*.gcp.gvt2.com', '*.url.google.com', '*.youtube-nocookie.com', '*.ytimg.com', 'android.com', '*.android.com', '*.flash.android.com', 'g.cn', '*.g.cn', 'g.co', '*.g.co', 'goo.gl', 'www.goo.gl', 'google-analytics.com', '*.google-analytics.com', 'google.com', 'googlecommerce.com', '*.googlecommerce.com', 'ggpht.cn', '*.ggpht.cn', 'urchin.com', '*.urchin.com', 'youtu.be', 'youtube.com', '*.youtube.com', 'youtubeeducation.com', '*.youtubeeducation.com', 'youtubekids.com', '*.youtubekids.com', 'yt.be', '*.yt.be', 'android.clients.google.com', 'developer.android.google.cn', 'developers.android.google.cn', 'source.android.google.cn']",
     "issuer": "GTS CA 1C3",
     "notBefore": "2021-08-23 01:38:08",
     "notAfter": "2021-11-15 01:38:07",
     "type": "Normal certificate type"
}
```
