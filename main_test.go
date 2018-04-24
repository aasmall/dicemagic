package main

import (
        "fmt"
        "google.golang.org/appengine/aetest"
        "gopkg.in/yaml.v2"
        "io/ioutil"
        "os"
        "testing"
)

func TestMain(m *testing.M) {
        var appYaml AppYaml
        appYaml.SetEnvironmentVariables()
        os.Exit(m.Run())
}

func (c *AppYaml) getAppYaml() *AppYaml {

        yamlFile, err := ioutil.ReadFile("app.yaml")
        if err != nil {
                fmt.Printf("yamlFile.Get err   #%v \n\n", err)
                return nil
        }
        err = yaml.Unmarshal(yamlFile, c)
        if err != nil {
                fmt.Printf("Unmarshal: %v\n\n", err)
                return nil
        }

        return c
}
func (c *AppYaml) SetEnvironmentVariables() *AppYaml {
        c.getAppYaml()
        os.Setenv("PROJECT_ID", c.AppYamlEnvVariables.PROJECTID)
        os.Setenv("KMSKEYRING", c.AppYamlEnvVariables.KMSKEYRING)
        os.Setenv("KMSKEY", c.AppYamlEnvVariables.KMSKEY)
        return c
}

func TestNaturalLanguageParsing(t *testing.T) {
        expression := "<@UAA6PDK7S> roll 1d4+12"
        result := parseMentionAndRoll(expression)
        if result <= 12 {
                t.Fatalf("failed to roll dice.")
        }
}
func TestComplexAttack(t *testing.T) {
        naturalLanguageAttack := "<@UAA6PDK7S> roll 1d4+1(Mundane)+1d8+5(Fire)-1d4(necrotic)"

        var attack = parseLanguageintoAttack(nil, naturalLanguageAttack)
        attack.totalDamage()
        fmt.Println(fmt.Sprintf("Attack: %+v", attack.totalDamage()))
        for _, e := range attack.DamageSegment {
                if !(e.diceResult > 0) {
                        t.Fatalf("Internal damage Segements not updating when rolling.")
                }
        }
}
func TestAddListAndDeleteIntegration(t *testing.T) {
        ctx, done, err := aetest.NewContext()
        if err != nil {
                t.Fatal(err)
        }
        defer done()
        db, err := configureDatastoreDB(ctx, os.Getenv("PROJECT_ID"))
        if err != nil {
                t.Fatalf("%+v", err)
        }
        var integration = new(Integration)
        var oAuthApprovalResponse = new(OAuthApprovalResponse)
        oAuthApprovalResponse.AccessToken = "AccessToken"
        oAuthApprovalResponse.Scope = "Scope"
        oAuthApprovalResponse.TeamName = "TeamName"
        oAuthApprovalResponse.TeamID = "TeamID"
        integration.OAuthApprovalResponse = *oAuthApprovalResponse
        id, err := db.AddIntegration(ctx, integration)
        if err != nil {
                t.Fatalf("Could not Add Integration")
        }
        integration.ID = id

        fmt.Println(fmt.Sprintf("Created Integration. ID: %+v", integration.ID))
        fmt.Println(fmt.Sprintf("Listing Integrations for TeamID : %s", oAuthApprovalResponse.TeamID))

        integrations, err := db.ListIntegrationsByTeam(ctx, oAuthApprovalResponse.TeamID)
        fmt.Println(fmt.Sprintf("Listed Integrations. Count: %+v", len(integrations)))

        integrationsAll, err := db.ListIntegrationsByTeam(ctx, "")
        fmt.Println(fmt.Sprintf("Listed All Integrations. Count: %+v", len(integrationsAll)))

        //err = db.DeleteIntegration(ctx, integration.ID)
        if err != nil {
                t.Fatalf("Could not Delete Integration: %+v", integration.ID)
        }
        fmt.Println(fmt.Sprintf("Deleted Integration. ID: %+v", integration.ID))
}
func TestEncryptDecrypt(t *testing.T) {
        ctx, done, err := aetest.NewContext()
        if err != nil {
                t.Fatal(err)
        }
        defer done()
        testString := "EncryptThis"
        fmt.Printf("testString: %v\n", testString)
        ciphertext := encrypt(testString, ctx)
        fmt.Printf("ciphertext: %v\n", ciphertext)
        plaintext := decrypt(ciphertext, ctx)
        fmt.Printf("plaintext: %v\n", plaintext)
        if !(plaintext == testString) {
                t.Fatalf("plaintext does not match testString: %+v", plaintext)
        }
}
