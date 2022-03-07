package config

import (
	"embed"
	"regexp"
	"strings"
	"testing"
	"time"

	gfs "github.com/gopherfs/fs"
	"github.com/gopherfs/fs/io/mem/simple"
	"github.com/kylelemons/godebug/pretty"
)

//go:embed config.toml
var f embed.FS

func TestFromFile(t *testing.T) {
	vals := map[string]string{
		"Subscription": `Subscription`,
		"Tenant":       `tenant`,
		"Region":       `region`,
	}

	wfs := simple.New()
	if err := gfs.Merge(wfs, f, ""); err != nil {
		panic(err)
	}
	got, err := FromFile(wfs, "config.toml", vals)
	if err != nil {
		panic(err)
	}

	want := &Config{
		Required: []Required{
			{Name: "Subscription", Regex: `^Subscription$`},
			{Name: "Tenant"},
			{Name: "Region"},
		},
		CreateVars: []*CreateVar{
			{Name: "Create KubeName", Key: "KubeName", Value: "kube_{{ .Region }}"},
			{Name: "Create MCResc", Key: "MCResc", Value: "MC_{{ .KubeName }}_{{ .KubeName }}_{{ .Region }}"},
			{Name: "Create UserMSI", Key: "UserMSI", Value: "{{ .Region }}-msi-ua"},
		},
		sequences: []*Sequence{
			{
				runner: &Runner{
					Name: "AzLogin",
					Cmd:  "az login --use-device-code",
				},
			},
			{
				runner: &Runner{
					Name: "SetAccount",
					Cmd:  "az account set -s {{.Subscription}}",
				},
			},
			{
				runner: &Runner{
					Name: "CreateGroup",
					Cmd:  "az group create --name {{ .KubeName }} --location {{ .Region }}",
				},
			},
			{
				runner: &Runner{
					Name:     "VnetCreate",
					Cmd:      "az network vnet create --name {{ .KubeName }} --resource-group {{ .KubeName }} --subnet-name default --subnet-prefix 10.0.0.0/16",
					ValueKey: "VnetInfo",
				},
			},
			{
				writeFile: &WriteFile{
					Name:  "Write Vnet Info",
					Path:  "./vnet.json",
					Value: "{{.VnetInfo}}",
				},
			},
			{
				runner: &Runner{
					Name:     "CreateCluster",
					Cmd:      `az aks create --name kube_{{.Region}} --resource-group {{ .KubeName }} --os-sku CBLMariner --max-pods 250 --network-plugin azure --vnet-subnet-id /subscriptions/{{ .Subscription }}/resourceGroups/{{ .Region }}/providers/Microsoft.Network/virtualNetworks/{{ .KubeName }}/subnets/default --docker-bridge-address 172.17.0.1/16 --service-cidr 10.2.0.0/24 --enable-managed-identity`,
					ValueKey: "ClusterInfo",
				},
			},
			{
				writeFile: &WriteFile{
					Name:  "Write Cluster Info",
					Path:  "./cluster.json",
					Value: "{{.ClusterInfo}}",
				},
			},
			{
				runner: &Runner{
					Name:     "SystemMSI",
					Cmd:      "az aks show -g {{ .KubeName }} -n {{ .KubeName }} --query identity",
					ValueKey: "SystemMSI",
				},
			},
			{
				writeFile: &WriteFile{
					Name:  "Write System MSI Info",
					Path:  "./system_msi.json",
					Value: "{{.SystemMSI}}",
				},
			},
			{
				runner: &Runner{
					Name: "GetCreds",
					Cmd:  "az aks get-credentials --resource-group {{ .KubeName }} --name {{ .KubeName }}",
				},
			},
			{
				runner: &Runner{
					Name:     "GetMSIID",
					Cmd:      `az aks show -g {{ .KubeName }} -n {{ .KubeName }} --query "identityProfile.kubeletidentity.clientId" -otsv`,
					ValueKey: "MSIID",
				},
			},
			{
				writeFile: &WriteFile{
					Name:  "Write System MSI Info 2",
					Path:  "./system_msi2.json",
					Value: "{{.MSIID}}",
				},
			},
			{
				runner: &Runner{
					Name: "RoleAssignmentManagedIdentity",
					Cmd:  `az role assignment create --role "Managed Identity Operator" --assignee {{ .MSIID }} --scope /subscriptions/{{ .Subscription }}/resourcegroups/{{ .MCResc }}`,
				},
			},
			{
				runner: &Runner{
					Name: "RoleAssignmentVirtualMachine",
					Cmd:  `az role assignment create --role "Virtual Machine Contributor" --assignee {{.MSIID}} --scope /subscriptions/{{.Subscription}}/resourcegroups/{{ .MCResc}}`,
				},
			},
			{
				runner: &Runner{
					Name: "AADPodDeploy",
					Cmd:  "kubectl apply -f https://raw.githubusercontent.com/Azure/aad-pod-identity/master/deploy/infra/deployment-rbac.yaml",
				},
			},
			{
				runner: &Runner{
					Name: "DeployMicAndAKSExceptions",
					Cmd:  "kubectl apply -f https://raw.githubusercontent.com/Azure/aad-pod-identity/master/deploy/infra/mic-exception.yaml",
				},
			},
			{
				runner: &Runner{
					Name: "CreateUserMSI",
					Cmd:  "az identity create -g {{ .KubeName }} -n {{ .UserMSI }}",
				},
			},
			{
				runner: &Runner{
					Name:       "GetUserMSIID",
					Cmd:        "az identity show -g {{ .KubeName }} -n {{ .UserMSI }} --query clientId -otsv",
					ValueKey:   "UserMSIID",
					RetrySleep: duration{1 * time.Minute},
					Retries:    5,
				},
			},
			{
				runner: &Runner{
					Name:     "GetUserMSIResc",
					Cmd:      "az identity show -g {{ .Region }} -n {{ .UserMSI }} --query id -otsv",
					ValueKey: "UserMSIResc",
				},
			},
			{
				runner: &Runner{
					Name: "AssignRoleReader",
					Cmd:  `az role assignment create --role Reader --assignee {{.UserMSIID}} --scope "/subscriptions/{{ .Subscription }}/resourceGroups/{{ .MCResc }}" --query id -otsv`,
				},
			},
			{
				writeFile: &WriteFile{
					Name: "Write aadident.yaml",
					Path: "./aadident.yaml",
					Value: strings.TrimSpace(`
apiVersion: "aadpodidentity.k8s.io/v1"
kind: AzureIdentity
metadata:
    name: {{ .UserMSI }}
spec:
    type: 0
    resourceID: {{ .UserMSIResc }}
    clientID: {{ .UserMSIID }}
`),
				},
			},
			{
				writeFile: &WriteFile{
					Name: "Write aadbinding.yaml",
					Path: "./aadbinding.yaml",
					Value: strings.TrimSpace(`
apiVersion: "aadpodidentity.k8s.io/v1"
kind: AzureIdentityBinding
metadata:
    name: {{ .UserMSI }}-binding
spec:
    azureIdentity: {{ .UserMSI }}
    selector: {{ .UserMSI }}
`),
				},
			},
			{
				runner: &Runner{
					Name: "Apply aadident.yaml",
					Cmd:  "kubectl apply -f aadident.yaml",
				},
			},
			{
				runner: &Runner{
					Name: "Apply aadbinding.yaml",
					Cmd:  "kubectl apply -f aadbinding.yaml",
				},
			},
		},
		required: map[string]*regexp.Regexp{
			"Subscription": regexp.MustCompile(`^Subscription$`),
			"Tenant":       nil,
			"Region":       nil,
		},
	}
	got.Seqs = nil // The original values before we translate to []Sequence aren't needed.

	pconf := pretty.Config{
		Diffable:          true,
		IncludeUnexported: true,
		SkipZeroFields:    true,
	}

	if diff := pconf.Compare(want, got); diff != "" {
		t.Errorf("TestFromFile: -want/+got:\n%s", diff)
	}

	mapHas(t, vals, "Subscription", "Subscription")
	mapHas(t, vals, "Region", "region")
	mapHas(t, vals, "Tenant", "tenant")
	mapHas(t, vals, "KubeName", "kube_region")
	mapHas(t, vals, "MCResc", "MC_kube_region_kube_region_region")
	mapHas(t, vals, "UserMSI", "region-msi-ua")
}

func mapHas(t *testing.T, vals map[string]string, name string, value string) {
	if v, ok := vals[name]; v != value {
		if !ok {
			t.Errorf("vals map does not have key(%s)", name)
			return
		}
		t.Errorf("vals map key(%s): got %q, want %q", name, vals[name], value)
	}
}
