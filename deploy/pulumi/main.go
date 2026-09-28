// Command pulumi-resource-markupp expõe o componente Markupp como pacote
// Pulumi, consumido com `pulumi package add` a partir deste diretório.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// version acompanha a tag do repositório. O release sobrescreve com -ldflags.
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
