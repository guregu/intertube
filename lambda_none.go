//go:build !lambda

package main

func startLambda() {
	panic("lambda disabled")
}

func startEventLambda(mode string) {
	panic("lambda disabled")
}
