package main

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	componentType       = "markupp:index:Markupp"
	defaultGatewayClass = "cilium"
	issuerACME          = "acme"
	issuerSelfSigned    = "selfSigned"
	defaultAPIReplicas  = 2
	defaultDBSize       = "10Gi"
	apiPort             = 8080
)

// Markupp é a instalação do markupp num namespace do Kubernetes: banco, job de
// migração, API e entrada com TLS.
type Markupp struct {
	pulumi.ResourceState

	URL pulumi.StringOutput `pulumi:"url"`
}

// MarkuppArgs são os parâmetros da instalação.
type MarkuppArgs struct {
	// Namespace já existente onde tudo é criado. Criar o namespace fica com
	// quem administra o cluster.
	Namespace string `pulumi:"namespace"`
	// Image é a imagem do servidor, com tag de versão.
	Image string `pulumi:"image"`
	// Host é o domínio público da API.
	Host string `pulumi:"host"`
	// CertificateIssuer é acme, com Let's Encrypt, ou selfSigned, para
	// cluster sem domínio público. Padrão acme.
	CertificateIssuer string `pulumi:"certificateIssuer,optional"`
	// AcmeEmail recebe os avisos do Let's Encrypt. Obrigatório com acme.
	AcmeEmail string `pulumi:"acmeEmail,optional"`
	// GatewayClassName é a classe do Gateway. Padrão cilium.
	GatewayClassName string `pulumi:"gatewayClassName,optional"`
	// ExistingGateway pendura o HTTPRoute num Gateway que já existe no
	// cluster, sem criar Gateway nem Issuer. O certificado fica com quem
	// administra esse Gateway.
	ExistingGateway *GatewayRefArgs `pulumi:"existingGateway,optional"`
	// APIReplicas é o número de réplicas da API. Padrão 2.
	APIReplicas int `pulumi:"apiReplicas,optional"`
	// Database define de onde vem o PostgreSQL.
	Database DatabaseArgs `pulumi:"database"`
}

// GatewayRefArgs aponta para um Gateway de outro namespace. Ele precisa
// aceitar rotas do namespace do markupp em allowedRoutes.
type GatewayRefArgs struct {
	Name      string `pulumi:"name"`
	Namespace string `pulumi:"namespace"`
	// SectionName é o listener do Gateway. Vazio deixa o Gateway escolher
	// pelo host.
	SectionName string `pulumi:"sectionName,optional"`
}

// DatabaseArgs escolhe entre um PostgreSQL externo e um PostgreSQL de uma
// instância dentro do namespace.
type DatabaseArgs struct {
	// ExternalURL aponta para um PostgreSQL já existente. Vazio sobe um
	// PostgreSQL no namespace, sem nada no nível do cluster.
	ExternalURL pulumi.StringInput `pulumi:"externalUrl,optional" provider:"secret"`
	// StorageClass do volume do PostgreSQL do namespace.
	StorageClass string `pulumi:"storageClass,optional"`
	// Size do volume do PostgreSQL do namespace. Padrão 10Gi.
	Size string `pulumi:"size,optional"`
}

// NewMarkupp declara a instalação completa sob o componente name.
func NewMarkupp(ctx *pulumi.Context, name string, args MarkuppArgs, opts ...pulumi.ResourceOption) (*Markupp, error) {
	if err := validate(args); err != nil {
		return nil, err
	}
	args = withDefaults(args)
	self := &Markupp{}
	if err := ctx.RegisterComponentResource(componentType, name, self, opts...); err != nil {
		return nil, err
	}
	if err := declareResources(ctx, name, args, pulumi.Parent(self)); err != nil {
		return nil, err
	}
	self.URL = pulumi.Sprintf("https://%s", args.Host)
	return self, ctx.RegisterResourceOutputs(self, pulumi.Map{"url": self.URL})
}

func declareResources(ctx *pulumi.Context, name string, args MarkuppArgs, parent pulumi.ResourceOption) error {
	names := newResourceNames(name)
	db, err := declareDatabase(ctx, names, args, parent)
	if err != nil {
		return err
	}
	migration, err := declareMigrationJob(ctx, names, args, db, parent)
	if err != nil {
		return err
	}
	if err := declareAPI(ctx, names, args, parent, pulumi.DependsOn([]pulumi.Resource{migration})); err != nil {
		return err
	}
	return declareIngress(ctx, names, args, parent)
}

type requiredField struct{ field, value string }

func validate(args MarkuppArgs) error {
	required := []requiredField{{"namespace", args.Namespace}, {"image", args.Image}, {"host", args.Host}}
	ingress, err := ingressRequiredFields(args)
	if err != nil {
		return err
	}
	for _, r := range append(required, ingress...) {
		if r.value == "" {
			return fmt.Errorf("campo %s vazio, esperado texto não vazio (namespace=%q image=%q host=%q)",
				r.field, args.Namespace, args.Image, args.Host)
		}
	}
	return nil
}

// ingressRequiredFields lista o que a entrada exige: com Gateway existente, a
// referência a ele; sem, o email do ACME quando o emissor é acme.
func ingressRequiredFields(args MarkuppArgs) ([]requiredField, error) {
	if ref := args.ExistingGateway; ref != nil {
		return []requiredField{{"existingGateway.name", ref.Name}, {"existingGateway.namespace", ref.Namespace}}, nil
	}
	switch args.CertificateIssuer {
	case "", issuerACME:
		return []requiredField{{"acmeEmail", args.AcmeEmail}}, nil
	case issuerSelfSigned:
		return nil, nil
	}
	return nil, fmt.Errorf("certificateIssuer=%q, esperado %q ou %q", args.CertificateIssuer, issuerACME, issuerSelfSigned)
}

func withDefaults(args MarkuppArgs) MarkuppArgs {
	if args.CertificateIssuer == "" {
		args.CertificateIssuer = issuerACME
	}
	if args.GatewayClassName == "" {
		args.GatewayClassName = defaultGatewayClass
	}
	if args.APIReplicas == 0 {
		args.APIReplicas = defaultAPIReplicas
	}
	if args.Database.Size == "" {
		args.Database.Size = defaultDBSize
	}
	return args
}

// resourceNames centraliza os nomes que um recurso usa para achar outro.
type resourceNames struct {
	component, database, dbSecret, api, gateway, issuer, tlsSecret string
}

func newResourceNames(component string) resourceNames {
	return resourceNames{
		component: component,
		database:  component + "-db",
		dbSecret:  component + "-db-app",
		api:       component + "-api",
		gateway:   component + "-gateway",
		issuer:    component + "-issuer",
		tlsSecret: component + "-tls",
	}
}
