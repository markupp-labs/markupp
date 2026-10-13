// Command pulumi-resource-markupp expõe o componente Markupp como pacote
// Pulumi, consumido com `pulumi package add` a partir deste diretório.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// version é a versão que o provider declara ao motor do Pulumi. Quem consome o
// componente fixa a versão pela tag do git em `pulumi package add ...@vX.Y.Z`,
// e o release não compila o componente, então aqui fica um valor de
// desenvolvimento.
var version = "0.0.0-dev"

func main() {
	provider, err := infer.NewProviderBuilder().
		WithComponents(infer.ComponentF(NewMarkupp)).
		Build()
	if err == nil {
		err = provider.Run(context.Background(), "markupp", version)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "provider markupp encerrado: %v\n", err)
		os.Exit(1)
	}
}
