package tui

import "testing"

func TestLayoutUsesStackedImplementationWhenSideBySideIsInactive(t *testing.T) {
	model := Model{rows: rowsFrom(codeRow(0))}

	if _, ok := model.layout().(stackedLayout); !ok {
		t.Fatalf("layout type = %T, want stackedLayout", model.layout())
	}
}

func TestLayoutUsesSideBySideProjectionWhenSideBySideIsActive(t *testing.T) {
	model := Model{
		rows:       rowsFrom(codeRow(0)),
		width:      minimumSideBySideWidth,
		sideBySide: true,
	}

	if _, ok := model.layout().(sideBySideProjection); !ok {
		t.Fatalf("layout type = %T, want sideBySideProjection", model.layout())
	}
}

func TestLayoutUsesStackedImplementationBelowSideBySideWidth(t *testing.T) {
	model := Model{
		rows:       rowsFrom(codeRow(0)),
		width:      minimumSideBySideWidth - 1,
		sideBySide: true,
	}

	if _, ok := model.layout().(stackedLayout); !ok {
		t.Fatalf("layout type = %T, want stackedLayout", model.layout())
	}
}
