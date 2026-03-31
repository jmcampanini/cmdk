package item

import (
	"testing"

	"charm.land/bubbles/v2/list"
)

var _ list.Item = Item{}

func TestNewItem_DataNotNil(t *testing.T) {
	i := NewItem()
	if i.Data == nil {
		t.Fatal("NewItem().Data should not be nil")
	}
}

func TestItem_FilterValue(t *testing.T) {
	i := Item{Display: "main:1 zsh", Type: "window"}

	if got := i.FilterValue(); got != "main:1 zsh" {
		t.Errorf("FilterValue() = %q, want %q", got, "main:1 zsh")
	}
}

func TestActionStaged_Constant(t *testing.T) {
	if ActionStaged != "staged" {
		t.Errorf("ActionStaged = %q, want %q", ActionStaged, "staged")
	}
}

func TestItem_StagesField(t *testing.T) {
	i := Item{
		Type:   "action",
		Action: ActionStaged,
		Stages: []Stage{
			{Type: StagePrompt, Key: "name", Text: "Enter name", Default: "world"},
			{Type: StagePicker, Key: "dir", Source: "zoxide"},
		},
	}
	if i.Type != "action" {
		t.Errorf("Type = %q, want action", i.Type)
	}
	if i.Action != ActionStaged {
		t.Errorf("Action = %q, want staged", i.Action)
	}
	if len(i.Stages) != 2 {
		t.Fatalf("Stages len = %d, want 2", len(i.Stages))
	}
	if i.Stages[0].Type != StagePrompt {
		t.Errorf("Stages[0].Type = %q, want prompt", i.Stages[0].Type)
	}
	if i.Stages[0].Key != "name" {
		t.Errorf("Stages[0].Key = %q, want name", i.Stages[0].Key)
	}
	if i.Stages[0].Text != "Enter name" {
		t.Errorf("Stages[0].Text = %q, want Enter name", i.Stages[0].Text)
	}
	if i.Stages[0].Default != "world" {
		t.Errorf("Stages[0].Default = %q, want world", i.Stages[0].Default)
	}
	if i.Stages[1].Type != StagePicker {
		t.Errorf("Stages[1].Type = %q, want picker", i.Stages[1].Type)
	}
	if i.Stages[1].Source != "zoxide" {
		t.Errorf("Stages[1].Source = %q, want zoxide", i.Stages[1].Source)
	}
}

func TestItem_NoStages(t *testing.T) {
	i := Item{Type: "action", Action: ActionExecute}
	if i.Type != "action" {
		t.Errorf("Type = %q, want action", i.Type)
	}
	if i.Action != ActionExecute {
		t.Errorf("Action = %q, want execute", i.Action)
	}
	if len(i.Stages) != 0 {
		t.Errorf("Stages len = %d, want 0", len(i.Stages))
	}
}

func TestStageType_Constants(t *testing.T) {
	if StagePrompt != "prompt" {
		t.Errorf("StagePrompt = %q, want prompt", StagePrompt)
	}
	if StagePicker != "picker" {
		t.Errorf("StagePicker = %q, want picker", StagePicker)
	}
}

func TestStage_EffectiveDelimiter(t *testing.T) {
	tests := []struct {
		name  string
		stage Stage
		want  string
	}{
		{"no fields configured", Stage{}, ""},
		{"explicit delimiter", Stage{Delimiter: ":"}, ":"},
		{"display only defaults to pipe", Stage{Display: 1}, "|"},
		{"pass only defaults to pipe", Stage{Pass: 2}, "|"},
		{"delimiter overrides default", Stage{Delimiter: "::", Display: 1, Pass: 2}, "::"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.stage.EffectiveDelimiter(); got != tt.want {
				t.Errorf("EffectiveDelimiter() = %q, want %q", got, tt.want)
			}
		})
	}
}
