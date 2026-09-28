package main

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	componentType       = "markupp:index:Markupp"
	defaultGatewayClass = "cilium"
	defaultAPIReplicas  = 2
	defaultDBInstances  = 2
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
	// AcmeEmail recebe os avisos do Let's Encrypt.
	AcmeEmail string `pulumi:"acmeEmail"`
	// GatewayClassName é a classe do Gateway. Padrão cilium.
	GatewayClassName string `pulumi:"gatewayClassName,optional"`
	// APIReplicas é o número de réplicas da API. Padrão 2.
	APIReplicas int `pulumi:"apiReplicas,optional"`
	// Database define de onde vem o PostgreSQL.
	Database DatabaseArgs `pulumi:"database"`
}

// DatabaseArgs escolhe entre um PostgreSQL externo e um cluster CloudNativePG.
type DatabaseArgs struct {
	// ExternalURL aponta para um PostgreSQL já existente. Vazio cria um
	// cluster CloudNativePG, que exige o operador instalado.
	ExternalURL pulumi.StringInput `pulumi:"externalUrl,optional" provider:"secret"`
	// Instances é o número de instâncias do CloudNativePG. Padrão 2.
	Instances int `pulumi:"instances,optional"`
	// StorageClass dos volumes do CloudNativePG.
	StorageClass string `pulumi:"storageClass,optional"`
	// Size de cada volume do CloudNativePG. Padrão 10Gi.
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

func validate(args MarkuppArgs) error {
	required := []struct{ field, value string }{
		{"namespace", args.Namespace}, {"image", args.Image}, {"host", args.Host}, {"acmeEmail", args.AcmeEmail},
	}
	for _, r := range required {
		if r.value == "" {
			return fmt.Errorf("campo %s vazio, esperado texto não vazio (namespace=%q image=%q host=%q)",
				r.field, args.Namespace, args.Image, args.Host)
		}
	}
	return nil
}

func withDefaults(args MarkuppArgs) MarkuppArgs {
	if args.GatewayClassName == "" {
		args.GatewayClassName = defaultGatewayClass
	}
	if args.APIReplicas == 0 {
		args.APIReplicas = defaultAPIReplicas
	}
	if args.Database.Instances == 0 {
		args.Database.Instances = defaultDBInstances
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
