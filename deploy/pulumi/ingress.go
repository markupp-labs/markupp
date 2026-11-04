package main

import (
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apiextensions"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	httpListener  = "http"
	httpsListener = "https"
)

// declareIngress cria Issuer, Gateway e HTTPRoute no namespace do markupp, ou
// só o HTTPRoute quando há um Gateway existente. O Issuer é do namespace, e
// não ClusterIssuer, para a credencial do CI não precisar de permissão no
// cluster inteiro.
func declareIngress(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, parent pulumi.ResourceOption) error {
	if args.ExistingGateway != nil {
		return declareRoutes(ctx, names, args, existingGatewayRef(*args.ExistingGateway), panelExistingGatewayRef(args), parent)
	}
	if err := declareIssuer(ctx, names, args, parent); err != nil {
		return err
	}
	if err := declareGateway(ctx, names, args, parent); err != nil {
		return err
	}
	return declareRoutes(ctx, names, args, gatewayRef(names, args, httpsListener), gatewayRef(names, args, panelListener), parent)
}

// withoutEmptyGateway trata o bloco de existingGateway vazio como ausente: o Pulumi
// YAML não omite propriedade, e a stack sem Gateway existente passa o bloco vazio.
func withoutEmptyGateway(args MarkuppArgs) MarkuppArgs {
	if args.ExistingGateway != nil && *args.ExistingGateway == (GatewayRefArgs{}) {
		args.ExistingGateway = nil
	}
	return args
}

func declareIssuer(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, parent pulumi.ResourceOption) error {
	spec := pulumi.Map{"selfSigned": pulumi.Map{}}
	if args.CertificateIssuer == issuerACME {
		spec = acmeIssuerSpec(names, args)
	}
	return declareCustom(ctx, "cert-manager.io/v1", "Issuer", names.issuer, args, spec, parent)
}

// acmeIssuerSpec resolve o desafio HTTP-01 pelo listener http do próprio
// Gateway, como o cert-manager faz com Gateway API.
func acmeIssuerSpec(names resourceNames, args MarkuppArgs) pulumi.Map {
	solver := pulumi.Map{"http01": pulumi.Map{"gatewayHTTPRoute": pulumi.Map{
		"parentRefs": pulumi.Array{gatewayRef(names, args, httpListener)},
	}}}
	return pulumi.Map{"acme": pulumi.Map{
		"server":              pulumi.String("https://acme-v02.api.letsencrypt.org/directory"),
		"email":               pulumi.String(args.AcmeEmail),
		"privateKeySecretRef": pulumi.Map{"name": pulumi.String(names.issuer + "-account")},
		"solvers":             pulumi.Array{solver},
	}}
}

func declareGateway(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, parent pulumi.ResourceOption) error {
	http := pulumi.Map{"name": pulumi.String(httpListener), "protocol": pulumi.String("HTTP"), "port": pulumi.Int(80)}
	listeners := pulumi.Array{http, httpsListenerSpec(httpsListener, args.Host, names.tlsSecret)}
	if args.Panel != nil {
		listeners = append(listeners, httpsListenerSpec(panelListener, args.Panel.Host, names.panelTLS))
	}
	gateway := &apiextensions.CustomResourceArgs{
		ApiVersion: pulumi.String("gateway.networking.k8s.io/v1"),
		Kind:       pulumi.String("Gateway"),
		Metadata: metav1.ObjectMetaArgs{
			Name: pulumi.String(names.gateway), Namespace: pulumi.String(args.Namespace),
			Annotations: pulumi.StringMap{"cert-manager.io/issuer": pulumi.String(names.issuer)},
		},
		OtherFields: kubernetes.UntypedArgs{"spec": pulumi.Map{
			"gatewayClassName": pulumi.String(args.GatewayClassName),
			"listeners":        listeners,
		}},
	}
	_, err := apiextensions.NewCustomResource(ctx, names.gateway, gateway, parent)
	return err
}

// httpsListenerSpec termina o TLS de um host com o certificado que o cert-manager
// emite para o listener.
func httpsListenerSpec(name, host, tlsSecret string) pulumi.Map {
	return pulumi.Map{
		"name": pulumi.String(name), "protocol": pulumi.String("HTTPS"), "port": pulumi.Int(443),
		"hostname": pulumi.String(host),
		"tls": pulumi.Map{
			"mode":            pulumi.String("Terminate"),
			"certificateRefs": pulumi.Array{pulumi.Map{"name": pulumi.String(tlsSecret)}},
		},
	}
}

// declareRoutes cria a rota da API e, com painel, a do host do painel.
func declareRoutes(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, apiGateway, panelGateway pulumi.Map, parent pulumi.ResourceOption) error {
	if err := declareRoute(ctx, names.api, args.Host, apiPort, args, apiGateway, parent); err != nil {
		return err
	}
	if args.Panel == nil {
		return nil
	}
	return declareRoute(ctx, names.panel, args.Panel.Host, panelPort, args, panelGateway, parent)
}

// declareRoute manda o host para o Service de mesmo nome da rota.
func declareRoute(ctx *pulumi.Context, service, host string, port int, args MarkuppArgs, gateway pulumi.Map, parent pulumi.ResourceOption) error {
	backend := pulumi.Map{"name": pulumi.String(service), "port": pulumi.Int(port)}
	return declareCustom(ctx, "gateway.networking.k8s.io/v1", "HTTPRoute", service, args, pulumi.Map{
		"parentRefs": pulumi.Array{gateway},
		"hostnames":  pulumi.StringArray{pulumi.String(host)},
		"rules":      pulumi.Array{pulumi.Map{"backendRefs": pulumi.Array{backend}}},
	}, parent)
}

// panelExistingGatewayRef pendura o painel no mesmo Gateway existente, no listener
// do host dele.
func panelExistingGatewayRef(args MarkuppArgs) pulumi.Map {
	if args.Panel == nil {
		return nil
	}
	ref := *args.ExistingGateway
	ref.SectionName = args.Panel.SectionName
	return existingGatewayRef(ref)
}

func gatewayRef(names resourceNames, args MarkuppArgs, listener string) pulumi.Map {
	return pulumi.Map{
		"name":        pulumi.String(names.gateway),
		"namespace":   pulumi.String(args.Namespace),
		"sectionName": pulumi.String(listener),
	}
}

func existingGatewayRef(ref GatewayRefArgs) pulumi.Map {
	gateway := pulumi.Map{"name": pulumi.String(ref.Name), "namespace": pulumi.String(ref.Namespace)}
	if ref.SectionName != "" {
		gateway["sectionName"] = pulumi.String(ref.SectionName)
	}
	return gateway
}

func declareCustom(ctx *pulumi.Context, apiVersion, kind, name string, args MarkuppArgs, spec pulumi.Map, parent pulumi.ResourceOption) error {
	_, err := apiextensions.NewCustomResource(ctx, name, &apiextensions.CustomResourceArgs{
		ApiVersion:  pulumi.String(apiVersion),
		Kind:        pulumi.String(kind),
		Metadata:    namespaced(name, args.Namespace),
		OtherFields: kubernetes.UntypedArgs{"spec": spec},
	}, parent)
	return err
}
