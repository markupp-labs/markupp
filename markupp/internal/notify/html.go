package notify

import (
	_ "embed"
	"fmt"
	"html/template"
	"strings"
)

//go:embed email.html.tmpl
var emailLayoutSource string

// emailLayout é o HTML comum dos emails. O html/template escapa o que vem de fora,
// como o nome do tenant que quem convida escolhe.
var emailLayout = template.Must(template.New("email").Parse(emailLayoutSource))

// emailView é o que o modelo mostra. Link e PanelURL vazios tiram o botão e o link
// da assinatura.
type emailView struct {
	Subject, Lead, Closing, Caution, Reason string
	Link, Button, Label, Token              string
	PanelURL, PanelHost                     string
}

func (l letter) html(token, panelURL string) string {
	view := emailView{
		Subject: l.subject, Lead: l.lead, Closing: l.closing(panelURL), Caution: l.caution, Reason: l.reasonLine(),
		Button: l.route.button, Label: l.route.label, Token: token,
	}
	if host := panelHost(panelURL); host != "" {
		view.Link, view.PanelURL, view.PanelHost = tokenURL(panelURL, l.route, token), panelURL, host
	}
	var out strings.Builder
	if err := emailLayout.Execute(&out, view); err != nil {
		// O modelo é fixo e o strings.Builder não falha: erro aqui é defeito do modelo.
		panic(fmt.Sprintf("render email %q: %v", l.subject, err))
	}
	return out.String()
}
