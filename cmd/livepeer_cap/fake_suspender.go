package main

// Implements common.Suspender
type fakeSuspender struct{}

func (fs *fakeSuspender) Suspended(orch string) int { return 0 }
