// Package notifier provides webhook-based alert notifications for expiring
// certificates.
//
// The Notifier periodically checks certificates and sends POST requests to a
// configured webhook URL when certificates approach expiration (days left at or
// below the threshold). Alerts are rate-limited per certificate to avoid spam.
//
// # Alert Payload
//
// The webhook receives a JSON payload with certificate details:
//
//	{
//		"hostname": "example.com",
//		"port": 443,
//		"commonName": "example.com",
//		"issuer": "Let's Encrypt",
//		"daysLeft": 10,
//		"threshold": 15,
//		"message": "Certificate expires in 10 days"
//	}
//
// # Rate Limiting
//
// Each certificate can only trigger one alert per configured interval (default:
// 5 minutes). This prevents alert fatigue when running in watch mode.
//
// # Usage
//
// Create a notifier and run it:
//
//	n := notifier.New(notifier.Config{
//		URL:       "https://hooks.example.com/alert",
//		Threshold: 15,
//		Interval:  5 * time.Minute,
//	}, checker, hosts)
//	n.Run(ctx)
package notifier
