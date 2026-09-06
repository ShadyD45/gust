package stats

import "testing"

func TestSpearmanRhoPerfect(t *testing.T) {
	x := []float64{1, 2, 3, 4, 5}
	y := []float64{2, 4, 6, 8, 10}
	rho, err := SpearmanRho(x, y)
	if err != nil {
		t.Fatal(err)
	}
	if rho < 0.999 {
		t.Fatalf("expected ~1, got %v", rho)
	}
}

func TestSpearmanRhoInverse(t *testing.T) {
	x := []float64{1, 2, 3, 4, 5}
	y := []float64{10, 8, 6, 4, 2}
	rho, err := SpearmanRho(x, y)
	if err != nil {
		t.Fatal(err)
	}
	if rho > -0.999 {
		t.Fatalf("expected ~-1, got %v", rho)
	}
}

func TestSpearmanRhoTies(t *testing.T) {
	x := []float64{1, 1, 2, 3}
	y := []float64{1, 2, 3, 4}
	rho, err := SpearmanRho(x, y)
	if err != nil {
		t.Fatal(err)
	}
	if rho < 0.7 {
		t.Fatalf("expected positive correlation with ties, got %v", rho)
	}
}
