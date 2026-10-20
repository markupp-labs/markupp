package vaultgraph

import (
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/graph"
)

// pageRankMaxIterations é o teto de iterações do PageRank: com d=0,85 a diferença
// cai a 0,85 por iteração, e em 100 ela já está abaixo de 1e-7 da inicial.
const pageRankMaxIterations = 100

// Convergence diz em quantas rodadas um cálculo iterativo parou e se parou no teto
// antes de convergir. No PageRank Iterations conta as iterações; no Louvain, os
// níveis, e Passes conta as passadas do nível que mais passadas fez.
type Convergence struct {
	Iterations int
	Passes     int
	Capped     bool
}

// pageRank é a iteração de potência de Page et al. (1998) sobre as listas de saída
// out, a partir do vetor uniforme: a massa da nota sem saída se espalha por todas, e
// o teleporte dá (1-d)/n a cada uma. Para quando a norma 2 da diferença entre duas
// iterações fica abaixo de tol, ou em maxIterations.
func pageRank(out [][]graph.Node, d, tol float64, maxIterations int) ([]float64, Convergence) {
	rank, next := make([]float64, len(out)), make([]float64, len(out))
	for i := range rank {
		rank[i] = 1 / float64(len(out))
	}
	for i := 1; i <= maxIterations; i++ {
		powerStep(out, rank, next, d)
		rank, next = next, rank
		if floats.Distance(rank, next, 2) < tol {
			return rank, Convergence{Iterations: i}
		}
	}
	return rank, Convergence{Iterations: maxIterations, Capped: true}
}

// powerStep grava em next uma iteração do PageRank a partir de rank.
func powerStep(out [][]graph.Node, rank, next []float64, d float64) {
	n := float64(len(out))
	dangling := 0.0
	clear(next)
	for u, targets := range out {
		if len(targets) == 0 {
			dangling += rank[u]
			continue
		}
		share := d * rank[u] / float64(len(targets))
		for _, v := range targets {
			next[v.ID()] += share
		}
	}
	floats.AddConst((1-d)/n+d*dangling/n, next)
}
