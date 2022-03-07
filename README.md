# Runme

So, occassionally I need to stream together a bunch of command line tools to do something in the cloud. So for example using the az cli tool and kubectl. I often need to stamp out things like kube clusters every year or two to upgrade versions and upgrade internal packages. However, the landscape will have changed and I need to mutate the procedure.

While I can build custom tools using exec.Cmd, what I find later is that the combination of stuff I was doing is hard to mutate with my changes.  So I need to input new steps or modify commands. I found doing it the traditional way to be kinda taxing.

You might be saying something like, why don't you use bash or powershell. Don't even get me started with powershell... I want something a little more repeatable, less error prone and kinda restartable. Can I do that in whatever shell thing, yes.  But it takes a lot more pain. Its the same reason I don't write software in shell script, you can... but should you? To me, shell is for when I need to run one program in a loop or string a very limited amount of things together (like no more than 80 characters).

This library + binary allows me to string together a bunch of commands, capture output to push into the next thing.  It can create variables from variables using text/template. It also does some basic resume functions.

This is useful for writing semi-repeatable tooling (semi because what you are doing is changed by the upstream, not by me) and allow rapid mutation to get to a desired state.  And importantly, I can recover from failure, make changes to internal data and restart the process where I left off.

A recovery file is written to the temp directory of the system whenever a problem occurs. This allows you to restart the process with the recovery file. The recovery file holds all the variables that have been written, so you can modify the data if needed. You can also modify the config.toml file and then replay from whatever step you need to.

## Generic config runner

In this mode, you basically just run the `runme` tool and pass the `--conf` pointing to a configuration file and a `--vals` passing a JSON map of required values.

Configs are TOML files.

We support a few directives:
  * Required - This details variables that must be passed before starting
    * Name - The name of the variable (must start with upper case)
    * Regex - (Optional) A regex the variable must match
  * CreateVars - Creates a variable with a name and value
    * Name - The name of the variable
    * Value - The value of the variable, which must be a string. Supports Go template replacement with any current variable that is currently set
  * Seqs - Represents a sequenced event. A sequence can do multiple types of actions.
    * Name - The name of the sequence, must be unique
    * Path - If set, indicates you are writing a value to a file
    * Cmd - If set, indicates you are issuing a command on the command line
    * Value - A string that supports Go template replacement. If Path is set, this is what is written to the file. If Cmd is set, this is the command that is run
    * ValueKey - Only used when Cmd is set, writes the output of the command to a variable. The command output has its space trimmed


Here is an example of a config that uses Azure CLI to build a Kubernetes cluster that has system and user MSIs, uses AADPod Identities and writes our various configs. 

```toml
[[Required]]
	Name = "Subscription"
[[Required]]
	Name = "Tenant"
[[Required]]
	Name = "Region"

[[CreateVars]]
	Name = "Create KubeResc"
	Key = "KubeResc"
	Value = "kube_{{ .Region }}"

[[CreateVars]]
	Name = "Create KubeName"
	Key = "KubeName"
	Value = "kube-{{ .Region }}"

[[CreateVars]]
	Name = "Create MCResc"
	Key = "MCResc"
	Value = "MC_{{ .KubeResc }}_{{ .KubeName }}_{{ .Region }}"

[[CreateVars]]
	Name = "Create UserMSI"
	Key = "UserMSI"
	Value = "{{ .Region }}-msi-ua"

[[Seqs]]
	Name = "AzLogin"
	Cmd = "az login --use-device-code"

[[Seqs]]
	Name = "SetAccount"
	Cmd = "az account set -s {{ .Subscription }}"

[[Seqs]]
	Name = "CreateGroup"
	Cmd = "az group create --name {{ .KubeResc }} --location {{ .Region }}"

[[Seqs]]
	Name = "VnetCreate"
	Cmd = "az network vnet create --name {{ .KubeResc }} --resource-group {{ .KubeResc }} --subnet-name default --subnet-prefix 10.0.0.0/16"
	ValueKey = "VnetInfo"

[[Seqs]]
	Name = "Write Vnet Info"
	Path = "./vnet.json"
	Value = "{{.VnetInfo}}"

[[Seqs]]
# --assign-identity /subscriptions/{{ .Subscription }}/resourcegroups/{{ .Region }}/providers/Microsoft.ManagedIdentity/userAssignedIdentities/westus2-msi-ua
	Name = "CreateCluster"
	Cmd = """
	az aks create
	--name {{ .KubeName }}
	--resource-group {{ .KubeResc }}
	--os-sku CBLMariner
	--max-pods 250
	--network-plugin azure
	--vnet-subnet-id /subscriptions/{{ .Subscription }}/resourceGroups/{{ .KubeResc }}/providers/Microsoft.Network/virtualNetworks/{{ .KubeResc }}/subnets/default
	--docker-bridge-address 172.17.0.1/16
	--dns-service-ip 10.2.0.10
	--service-cidr 10.2.0.0/24
	--enable-managed-identity
	--yes
	"""
	ValueKey = "ClusterInfo"

[[Seqs]]
	Name = "Write Cluster Info"
	Path = "./cluster.json"
	Value = "{{.ClusterInfo}}"

[[Seqs]]
	Name = "SystemMSI"
	Cmd = "az aks show -g {{ .KubeResc }} -n {{ .KubeName }} --query identity"
	ValueKey = "SystemMSI"

[[Seqs]]
	Name = "Write System MSI Info"
	Path = "./system_msi.json"
	Value = "{{.SystemMSI}}"

[[Seqs]]
	Name = "GetCreds"
	Cmd = "az aks get-credentials --resource-group {{ .KubeResc }} --name {{ .KubeName }}"

# This isn't working, fix it.
[[Seqs]]
	Name = "GetMSIID"
	Cmd = "az aks show -g {{ .KubeResc }} -n {{ .KubeName }} --query \"identityProfile.kubeletidentity.clientId\" -otsv"
	ValueKey = "MSIID"

[[Seqs]]
	Name = "Write System MSI Info 2"
	Path = "./system_msi2.json"
	Value = "{{.MSIID}}"

[[Seqs]]
	Name = "RoleAssignmentManagedIdentity"
	Cmd = """
	az role assignment create 
	--role "Managed Identity Operator" 
	--assignee {{ .MSIID }} 
	--scope /subscriptions/{{ .Subscription }}/resourcegroups/{{ .KubeResc }}
	"""

[[Seqs]]
	Name = "RoleAssignmentVirtualMachine"
	Cmd = """
	az role assignment create
	--role "Virtual Machine Contributor"
	--assignee {{.MSIID}}
	--scope /subscriptions/{{.Subscription}}/resourcegroups/{{ .MCResc}}
	"""

[[Seqs]]
	Name = "AADPodDeploy"
	Cmd = "kubectl apply -f https://raw.githubusercontent.com/Azure/aad-pod-identity/master/deploy/infra/deployment-rbac.yaml"

[[Seqs]]
	Name = "DeployMicAndAKSExceptions"
	Cmd = "kubectl apply -f https://raw.githubusercontent.com/Azure/aad-pod-identity/master/deploy/infra/mic-exception.yaml"

[[Seqs]]
	Name = "CreateUserMSI"
	Cmd = "az identity create -g {{ .KubeResc }} -n {{ .UserMSI }}"

[[Seqs]]
	Name = "GetUserMSIID"
	Cmd = "az identity show -g {{ .KubeResc }} -n {{ .UserMSI }} --query clientId -otsv"
	ValueKey = "UserMSIID"
	RetrySleep = "1m"
	Retries = 5

[[Seqs]]
	Name = "GetUserMSIResc"
	Cmd = "az identity show -g {{ .KubeResc }} -n {{ .UserMSI }} --query id -otsv"
	ValueKey = "UserMSIResc"

[[Seqs]]
	Name = "AssignRoleReader"
	Cmd = """
	az role assignment create
	--role Reader
	--assignee {{.UserMSIID}}
	--scope "/subscriptions/{{ .Subscription }}/resourcegroups/{{ .KubeResc }}"
	--query id
	-otsv
	"""

[[Seqs]]
	Name = "Write aadident.yaml"
	Path = "./aadident.yaml"
	Value = """
apiVersion: "aadpodidentity.k8s.io/v1"
kind: AzureIdentity
metadata:
    name: {{ .UserMSI }}
spec:
    type: 0
    resourceID: {{ .UserMSIResc }}
    clientID: {{ .UserMSIID }}
  """

[[Seqs]]
	Name = "Write aadbinding.yaml"
	Path = "./aadbinding.yaml"
	Value = """
apiVersion: "aadpodidentity.k8s.io/v1"
kind: AzureIdentityBinding
metadata:
    name: {{ .UserMSI }}-binding
spec:
    azureIdentity: {{ .UserMSI }}
    selector: {{ .UserMSI }}
    """

[[Seqs]]
	Name = "Apply aadident.yaml"
	Cmd = "kubectl apply -f aadident.yaml"

[[Seqs]]
	Name = "Apply aadbinding.yaml"
	Cmd = "kubectl apply -f aadbinding.yaml"```
