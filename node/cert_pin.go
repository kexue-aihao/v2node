package node

import (
	"context"

	log "github.com/sirupsen/logrus"
	panel "github.com/wyx2685/v2node/api/v2board"
	"github.com/wyx2685/v2node/common/certutil"
)

func (c *Controller) reportCertPin(ctx context.Context) {
	if c.info == nil || c.info.Security != panel.Tls {
		return
	}
	if c.info.Common == nil || c.info.Common.CertInfo == nil {
		return
	}
	cert := c.info.Common.CertInfo
	if cert.CertMode == "none" || cert.CertFile == "" {
		return
	}
	pin, err := certutil.LeafSHA256HexFromFile(cert.CertFile)
	if err != nil {
		log.WithFields(log.Fields{
			"tag":  c.tag,
			"file": cert.CertFile,
			"err":  err,
		}).Warn("Read cert pin failed")
		return
	}
	if err := c.apiClient.ReportCertPin(ctx, pin); err != nil {
		log.WithFields(log.Fields{
			"tag": c.tag,
			"err": err,
		}).Warn("Report cert pin failed")
		return
	}
	log.WithFields(log.Fields{
		"tag": c.tag,
		"pin": pin,
	}).Info("Cert pin reported to panel")
}
