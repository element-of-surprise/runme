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
				{ItemWord, `"Managed Identity Operator"`},
				{ItemFlag, "--assignee"},
				{ItemWord, "63af9c3f-7503-4026-85b0-0b7a97f578f8"},
				{ItemFlag, "--scope"},
				{ItemWord, "/subscriptions/b0901439-755b-40bb-a7b0-672b3h76727a/resourceGroups/westus2"},
			},
		},
		{
			desc: "Line with {{ }}",
			line: `az aks create --name kube_{{.Region}} --resource-group {{ .KubeName }} --os-sku CBLMariner --max-pods 250 --network-plugin azure --vnet-subnet-id /subscriptions/{{ .Subscription }}/resourceGroups/{{ .Region }}/providers/Microsoft.Network/virtualNetworks/{{ .KubeName }}/subnets/default --docker-bridge-address 172.17.0.1/16 --service-cidr 10.2.0.0/24 --enable-managed-identity`,
			want: []Item{
				{ItemWord, "az"},
				{ItemWord, "aks"},
				{ItemWord, "create"},
				{ItemFlag, "--name"},
				{ItemWord, "kube_{{.Region}}"},
				{ItemFlag, "--resource-group"},
				{ItemWord, "{{ .KubeName }}"},
				{ItemFlag, "--os-sku"},
				{ItemWord, "CBLMariner"},
				{ItemFlag, "--max-pods"},
				{ItemWord, "250"},
				{ItemFlag, "--network-plugin"},
				{ItemWord, "azure"},
				{ItemFlag, "--vnet-subnet-id"},
				{ItemWord, "/subscriptions/{{ .Subscription }}/resourceGroups/{{ .Region }}/providers/Microsoft.Network/virtualNetworks/{{ .KubeName }}/subnets/default"},
				{ItemFlag, "--docker-bridge-address"},
				{ItemWord, "172.17.0.1/16"},
				{ItemFlag, "--service-cidr"},
				{ItemWord, "10.2.0.0/24"},
				{ItemFlag, "--enable-managed-identity"},
			},
		},
	}

	l := New()
	for _, test := range tests {
		ch := l.Parse(test.line)
		got := []Item{}
		for item := range ch {
			got = append(got, item)
		}
		if diff := pretty.Compare(test.want, got); diff != "" {
			t.Errorf("TestLine(%s): -want/+got:\n", diff)
		}
	}
}

/*
"az aks create --name kube_{{.Region}} --resource-group {{ .KubeName }} --os-sku CBLMariner --max-pods 250 --network-plugin azure --vnet-subnet-id /subscriptions/{{ .Subscription }}/resourceGroups/{{ .Region }}/providers/Microsoft.Network/virtualNetworks/{{ .KubeName }}/subnets/default --docker-bridge-address 172.17.0.1/16 --service-cidr 10.2.0.0/24 --enable-managed-identity"
*/

/*
func TestUntilClose(t *testing.T) {
	tests := []struct {
		desc string
		line string
		want string
	}{
		{
			desc: "Mustache {{ }}",
			line: `{{ .Hello }} my baby`,
			want: `{{ .Hello }}`,
		},
		{
			desc: "Mustache with no spaces",
			line: `{{.VnetInfo}}`,
			want: `{{.VnetInfo}}`,
		},
	}

	l := New()
	for _, test := range tests {
		l.pos, l.width, l.input = 0, 0, strings.TrimSpace(test.line)
		got, err := l.untilClose()
		if err != nil {
			panic(err)
		}
		if diff := pretty.Compare(test.want, got); diff != "" {
			t.Errorf("TestUntilClose(%s): -want/+got:\n%s", test.desc, diff)
		}
	}
}
*/
