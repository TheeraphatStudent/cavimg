package rewrite

import (
	"strings"
	"testing"
)

func TestTransformHTMLImg(t *testing.T) {
	out, changed := Transform("index.html", `<img src="a.png" alt="a">`)
	if !changed {
		t.Fatal("expected changed = true")
	}
	if out != `<cav-img src="a.png" alt="a"></cav-img>` {
		t.Fatalf("got %q", out)
	}
}

func TestTransformJSXSelfClosing(t *testing.T) {
	out, _ := Transform("App.tsx", `<img src={hero} alt="h" />`)
	if out != `<cav-img src={hero} alt="h" />` {
		t.Fatalf("got %q", out)
	}
}

func TestTransformIsIdempotent(t *testing.T) {
	src := `<img src="a.png" alt="a">`
	once, _ := Transform("index.html", src)
	twice, changed := Transform("index.html", once)
	if changed {
		t.Error("second transform should report no change")
	}
	if twice != once {
		t.Errorf("not idempotent: %q != %q", twice, once)
	}
}

func TestTransformIgnoresSvgImageAndCavImg(t *testing.T) {
	src := `<image href="a.png"/><cav-img src="b.png"></cav-img>`
	out, changed := Transform("index.html", src)
	if changed || out != src {
		t.Errorf("should not touch <image> or <cav-img>: %q", out)
	}
}

func TestWiring(t *testing.T) {
	if _, manual := Wiring("Vite+React"); manual {
		t.Error("Vite+React should not be manual")
	}
	steps, manual := Wiring("Vue")
	if !manual {
		t.Error("Vue should be manual")
	}
	if len(steps) == 0 {
		t.Error("expected guidance steps even when manual")
	}
	nextSteps, _ := Wiring("Next.js")
	if !strings.Contains(strings.Join(nextSteps, "\n"), "useEffect") {
		t.Error("Next.js guidance should mention useEffect")
	}
}
