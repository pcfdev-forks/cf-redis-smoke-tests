package context_setup

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	. "github.com/onsi/ginkgo"
	ginkgoconfig "github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

type ConfiguredContext struct {
	config IntegrationConfig

	organizationName string
	spaceName        string

	quotaDefinitionName string
	quotaDefinitionGUID string

	regularUserUsername string
	regularUserPassword string

	securityGroupName string

	isPersistent bool
}

type quotaDefinition struct {
	Name string `json:"name"`

	NonBasicServicesAllowed bool `json:"non_basic_services_allowed"`

	TotalServices int `json:"total_services"`
	TotalRoutes   int `json:"total_routes"`

	MemoryLimit int `json:"memory_limit"`
}

func NewContext(config IntegrationConfig, prefix string) *ConfiguredContext {
	node := ginkgoconfig.GinkgoConfig.ParallelNode
	timeTag := time.Now().Format("2006_01_02-15h04m05.999s")

	return &ConfiguredContext{
		config: config,

		quotaDefinitionName: fmt.Sprintf("%s-QUOTA-%d-%s", prefix, node, timeTag),

		organizationName: fmt.Sprintf("%s-ORG-%d-%s", prefix, node, timeTag),
		spaceName:        fmt.Sprintf("%s-SPACE-%d-%s", prefix, node, timeTag),

		regularUserUsername: fmt.Sprintf("%s-USER-%d-%s", prefix, node, timeTag),
		regularUserPassword: "6S5CURVjP7.5.05Ip61YxZph.65Tuv4.rCsXYP7A0.G1..tT8fhhhhhhcf......",

		securityGroupName: fmt.Sprintf("%s-SECURITY_GROUP-%d-%s", prefix, node, timeTag),

		isPersistent: false,
	}
}

func (context *ConfiguredContext) Setup() {
	cf.AsUser(context.AdminUserContext(), func() {
		channel := cf.Cf("create-user", context.regularUserUsername, context.regularUserPassword)
		select {
		case <-channel.Out.Detect("OK"):
		case <-channel.Out.Detect("scim_resource_already_exists"):
		case <-time.After(ScaledTimeout(10 * time.Second)):
			Fail("failed to create user")
		}

		definition := quotaDefinition{
			Name: context.quotaDefinitionName,

			TotalServices: 100,
			TotalRoutes:   1000,

			MemoryLimit: 10240,

			NonBasicServicesAllowed: true,
		}

		definitionPayload, err := json.Marshal(definition)
		Expect(err).ToNot(HaveOccurred())

		var response cf.GenericResource

		cf.ApiRequest("POST", "/v2/quota_definitions", &response, string(definitionPayload))

		context.quotaDefinitionGUID = response.Metadata.Guid

		Eventually(cf.Cf("create-org", context.organizationName), ScaledTimeout(60*time.Second)).Should(Exit(0))
		Eventually(cf.Cf("set-quota", context.organizationName, definition.Name), ScaledTimeout(60*time.Second)).Should(Exit(0))

		setUpSpaceWithUserAccess(context.RegularUserContext())

		if context.config.CreatePermissiveSecurityGroup {
			context.createPermissiveSecurityGroup()
		}
	})
}

func (context *ConfiguredContext) Teardown() {
	cf.AsUser(context.AdminUserContext(), func() {
		Eventually(cf.Cf("delete-user", "-f", context.regularUserUsername), ScaledTimeout(60*time.Second)).Should(Exit(0))

		if !context.isPersistent {
			Eventually(cf.Cf("delete-org", "-f", context.organizationName), ScaledTimeout(60*time.Second)).Should(Exit(0))

			cf.ApiRequest(
				"DELETE",
				"/v2/quota_definitions/"+context.quotaDefinitionGUID+"?recursive=true",
				nil,
			)
		}

		if context.config.CreatePermissiveSecurityGroup {
			Eventually(cf.Cf("delete-security-group", "-f", context.securityGroupName), ScaledTimeout(60*time.Second)).Should(Exit(0))
		}
	})
}

func (context *ConfiguredContext) AdminUserContext() cf.UserContext {
	return cf.NewUserContext(
		context.config.ApiEndpoint,
		context.config.AdminUser,
		context.config.AdminPassword,
		"",
		"",
		context.config.SkipSSLValidation,
	)
}

func (context *ConfiguredContext) RegularUserContext() cf.UserContext {
	return cf.NewUserContext(
		context.config.ApiEndpoint,
		context.regularUserUsername,
		context.regularUserPassword,
		context.organizationName,
		context.spaceName,
		context.config.SkipSSLValidation,
	)
}

func (context *ConfiguredContext) createPermissiveSecurityGroup() {
	rules := []map[string]string{
		map[string]string{
			"destination": "0.0.0.0-255.255.255.255",
			"protocol":    "all",
		},
	}

	rulesFile, err := ioutil.TempFile("", fmt.Sprintf("%s-rules.json", context.securityGroupName))
	Expect(err).ToNot(HaveOccurred())
	bytes, err := json.Marshal(rules)
	Expect(err).ToNot(HaveOccurred())
	_, err = rulesFile.Write(bytes)
	Expect(err).ToNot(HaveOccurred())
	rulesFile.Close()

	Eventually(cf.Cf("create-security-group", context.securityGroupName, rulesFile.Name()), ScaledTimeout(60*time.Second)).Should(Exit(0))
	Eventually(cf.Cf("bind-security-group", context.securityGroupName, context.organizationName, context.spaceName), ScaledTimeout(60*time.Second)).Should(Exit(0))
}
