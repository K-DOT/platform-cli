package nsx

import (
	"fmt"
	"strings"

	"github.com/fatih/structs"
	log "github.com/sirupsen/logrus"

	"github.com/flanksource/commons/certs"
	"github.com/flanksource/commons/console"
	"github.com/moshloop/platform-cli/pkg/platform"
)

const (
	Namespace = "nsx-system"
	CertName  = "nsx-secret"
)

func Install(p *platform.Platform) error {
	if p.NSX == nil || p.NSX.Disabled {
		return nil
	}

	if err := p.CreateOrUpdateNamespace(Namespace, nil, nil); err != nil {
		return err
	}

	if !p.HasSecret(Namespace, CertName) {
		cert := certs.NewCertificateBuilder("kubernetes-client").Certificate
		cert, err := p.GetCA().SignCertificate(cert, 10)
		if err != nil {
			return err
		}

		if err := p.CreateOrUpdateSecret(CertName, Namespace, cert.AsTLSSecret()); err != nil {
			return err
		}
	}

	// p.NSX.NsxV3.NsxApiCertFile = "/etc/nsx-ujo/nsx-cert/tls.crt"
	// p.NSX.NsxV3.NsxApiPrivateKeyFile = "/etc/nsx-ujo/nsx-cert/tls.key"
	yes := true
	p.NSX.NsxV3.Insecure = &yes
	p.NSX.NsxCOE.Cluster = p.Name

	ini := structs.Map(p.NSX)

	s := "[Defaults]\n" + mapToINI(ini)

	log.Tracef("Using NSX config: %s", console.StripSecrets(string(s)))

	if err := p.CreateOrUpdateConfigMap("nsx-ncp-config", Namespace, map[string]string{
		"ncp.ini": string(s),
	}); err != nil {
		return err
	}

	if err := p.CreateOrUpdateConfigMap("nsx-node-agent-config", Namespace, map[string]string{
		"ncp.ini": string(s),
	}); err != nil {
		return err
	}

	p.NSX.Image = p.GetImagePath("library/nsx-ncp-ubuntu:" + p.NSX.Version)
	if err := p.ApplySpecs(Namespace, "nsx.yaml"); err != nil {
		return err
	}

	return nil
}

func mapToINI(ini map[string]interface{}) string {
	s := ""
	for k, v := range ini {
		if v == nil {
			continue
		}
		switch v.(type) {
		case string:
			if v != "" {
				s += fmt.Sprintf("%s = %s\n", k, v)
			}
		case *int:
			i := v.(*int)
			s += fmt.Sprintf("%s = %v\n", k, *i)
		case *bool:
			b := v.(*bool)
			if b != nil && *b {
				s += fmt.Sprintf("%s = True\n", k)
			} else if b != nil {
				s += fmt.Sprintf("%s = False\n", k)
			}
		case []string:
			items := v.([]string)
			s += fmt.Sprintf("%s = %s\n", k, strings.Join(items, ","))
		case map[string]interface{}:
			s += fmt.Sprintf("[%s]\n%s\n", k, mapToINI(v.(map[string]interface{})))
		}
	}
	return s
}
