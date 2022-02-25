package lexer

import (
	"testing"

	"github.com/kylelemons/godebug/pretty"
)

func TestLine(t *testing.T) {
	tests := []struct {
		desc string
		line string
		want []Item
	}{
		{
			desc: "Line with double quotes",
			line: `az role assignment create --role "Managed Identity Operator" --assignee 63af9c3f-7503-4026-85b0-0b7a97f578f8 --scope /subscriptions/b0901439-755b-40bb-a7b0-672b3h76727a/resourceGroups/westus2`,
			want: []Item{
				{ItemWord, "az"},
				{ItemWord, "role"},
				{ItemWord, "assignment"},
				{ItemWord, "create"},
				{ItemFlag, "--role"},
				{ItemWord, "\"Managed Identity Operator\""},
				{ItemFlag, "--assignee"},
				{ItemWord, "63af9c3f-7503-4026-85b0-0b7a97f578f8"},
				{ItemFlag, "--scope"},
				{ItemWord, "/subscriptions/b0901439-755b-40bb-a7b0-672b3h76727a/resourceGroups/westus2"},
			},
		},
	}

	l := New("")
	for _, test := range tests {
		l.Reset(test.line)
		ch := l.Parse()
		got := []Item{}
		for item := range ch {
			got = append(got, item)
		}
		if diff := pretty.Compare(test.want, got); diff != "" {
			t.Errorf("TestLine(%s): -want/+got:\n", diff)
		}
	}
}
