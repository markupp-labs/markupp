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
		return declareRoute(ctx, names, args, existingGatewayRef(*args.ExistingGateway), parent)
	}
	if err := declareIssuer(ctx, names, args, parent); err != nil {
		return err
	}
	if err := declareGateway(ctx, names, args, parent); err != nil {
		return err
	}
	return declareRoute(ctx, names, args, gatewayRef(names, args, httpsListener), parent)
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
	https := pulumi.Map{
		"name": pulumi.String(httpsListener), "protocol": pulumi.String("HTTPS"), "port": pulumi.Int(443),
		"hostname": pulumi.String(args.Host),
		"tls": pulumi.Map{
			"mode":            pulumi.String("Terminate"),
			"certificateRefs": pulumi.Array{pulumi.Map{"name": pulumi.String(names.tlsSecret)}},
		},
	}
	http := pulumi.Map{"name": pulumi.String(httpListener), "protocol": pulumi.String("HTTP"), "port": pulumi.Int(80)}
	gateway := &apiextensions.CustomResourceArgs{
		ApiVersion: pulumi.String("gateway.networking.k8s.io/v1"),
		Kind:       pulumi.String("Gateway"),
		Metadata: metav1.ObjectMetaArgs{
			Name: pulumi.String(names.gateway), Namespace: pulumi.String(args.Namespace),
			Annotations: pulumi.StringMap{"cert-manager.io/issuer": pulumi.String(names.issuer)},
		},
		OtherFields: kubernetes.UntypedArgs{"spec": pulumi.Map{
			"gatewayClassName": pulumi.String(args.GatewayClassName),
			"listeners":        pulumi.Array{http, https},
		}},
	}
	_, err := apiextensions.NewCustomResource(ctx, names.gateway, gateway, parent)
	return err
}

func declareRoute(ctx *pulumi.Context, names resourceNames, args MarkuppArgs, gateway pulumi.Map, parent pulumi.ResourceOption) error {
	backend := pulumi.Map{"name": pulumi.String(names.api), "port": pulumi.Int(apiPort)}
	return declareCustom(ctx, "gateway.networking.k8s.io/v1", "HTTPRoute", names.api, args, pulumi.Map{
		"parentRefs": pulumi.Array{gateway},
		"hostnames":  pulumi.StringArray{pulumi.String(args.Host)},
		"rules":      pulumi.Array{pulumi.Map{"backendRefs": pulumi.Array{backend}}},
	}, parent)
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
