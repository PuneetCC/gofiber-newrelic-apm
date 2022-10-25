package newrelicapm

import (
	"fmt"
	"net/url"

	"github.com/gofiber/fiber/v2"
	"github.com/newrelic/go-agent/v3/newrelic"
)

type Config struct {
	// License parameter is required to initialize newrelic application
	License string
	// AppName parameter is required to initialize newrelic application, default is fiber-api
	AppName string
	// Enabled parameter passed to enable/disable newrelic
	Enabled bool
	// TransportType can be HTTP or HTTPS (case-sensitive), default is HTTP
	TransportType string
}

var ConfigDefault = Config{
	License:       "",
	AppName:       "fiber-api",
	Enabled:       false,
	TransportType: string(newrelic.TransportHTTP),
}

var txLocalTag = "newrelic-apm-tx"

func getTX(c *fiber.Ctx) *newrelic.Transaction {
	tx, ok := c.Locals(txLocalTag).(*newrelic.Transaction)
	if !ok {
		return nil
	}
	return tx
}

func SetLabel(c *fiber.Ctx, key, value string) {
	tx := getTX(c)
	if tx != nil {
		tx.AddAttribute("labels."+key, value)
	}
}

func StartSpan(c *fiber.Ctx, name string) *newrelic.Segment {
	tx := getTX(c)
	if tx == nil {
		return nil
	}
	span := tx.StartSegment(name)
	return span
}

func Error(c *fiber.Ctx, err error) {
	tx := getTX(c)
	if tx != nil {
		tx.NoticeError(err)
	}
}

func noop(c *fiber.Ctx) error {
	return nil
}

func New(cfg Config) fiber.Handler {
	if cfg.TransportType != "HTTP" && cfg.TransportType != "HTTPS" {
		cfg.TransportType = ConfigDefault.TransportType
	}

	if cfg.AppName == "" {
		cfg.AppName = ConfigDefault.AppName
	}

	if cfg.License == "" {
		fmt.Println("unable to create New Relic Application -> License can not be empty")
		return noop
	}

	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName(cfg.AppName),
		newrelic.ConfigLicense(cfg.License),
		newrelic.ConfigEnabled(cfg.Enabled),
	)

	if err != nil {
		fmt.Println("unable to create New Relic Application -> %w", err)
		return noop
	}

	return func(c *fiber.Ctx) error {
		txn := app.StartTransaction(c.Method() + " " + c.Path())
		originalURL, err := url.Parse(c.OriginalURL())
		if err != nil {
			return c.Next()
		}
		c.Locals(txLocalTag, txn)

		txn.SetWebRequest(newrelic.WebRequest{
			URL:       originalURL,
			Method:    c.Method(),
			Transport: newrelic.TransportType(cfg.TransportType),
			Host:      c.Hostname(),
		})

		err = c.Next()
		if err != nil {
			txn.NoticeError(err)
		}

		defer txn.SetWebResponse(nil).WriteHeader(c.Response().StatusCode())
		defer txn.End()
		return err
	}
}
