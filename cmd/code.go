package cmd

import (
	"github.com/spf13/cobra"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"os"
)

type Field struct {
    Name      string `json:"name"`
    Attribute string `json:"attribute"`
	Prompt    string `json:"prompt"`
}

type BoilerPlateFile struct {
    Name   string `json:"name"`
    Fields []Field `json:"fields"`
}
type BoilerPlate struct {
	Files []BoilerPlateFile `json:"files"`
}

var codeCmd = &cobra.Command{
	Use:   "code",
	Short: "Code Generation",
}

var (
	generateCodeCmd = &cobra.Command{
		Use:          "generate",
		Short:        "Generate code from boilerplate",
		Args: 		  cobra.ArbitraryArgs,
		RunE:         GenerateCode,
		SilenceUsage: true,
	}
)

func GenerateCode(cmd *cobra.Command, args []string) error {
	var boilerplate BoilerPlate

	name, _ := cmd.Flags().GetString("name")
	tpe, _ := cmd.Flags().GetString("type")
	lang, _ := cmd.Flags().GetString("lang")
	bpl, _ := cmd.Flags().GetString("bpl")
	
	fmt.Println("Generating code...")
	
	// Creating project directory
	fmt.Println("Creating directory...")
	err := os.Mkdir(name, 0755)
	if err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Println("Directory created successfully!")
	
	// Loading Boilerplate configuration
	fmt.Println("Loading Boilerplate configuration file...")
	response, errget := http.Get("https://raw.githubusercontent.com/conductor-sdk/boilerplates/main/"+lang+"/"+tpe+"/"+bpl+"/bp.json")
   
	if errget != nil {
		fmt.Println(errget)
		return errget
	}
	
	body, errbody := ioutil.ReadAll(response.Body)
	if errbody != nil {
		fmt.Println(errbody)
		return errbody
	}
	// close response body
	response.Body.Close()
	
	errparse := json.Unmarshal(body, &boilerplate)

	if errparse != nil {
		fmt.Println(errparse)
		return errparse
	}
	fmt.Println("Boilerplate configuration loaded successfully!")


	for _, element := range boilerplate.Files {
		fmt.Println("Generating " + element.Name + " file...")
		resFile, errGetFile := http.Get("https://raw.githubusercontent.com/conductor-sdk/boilerplates/main/"+lang+"/"+tpe+"/"+bpl+"/"+element.Name)
		if errGetFile != nil {
			fmt.Println(errGetFile)
			return errGetFile
		}

		bodyFile, errbodyFile := ioutil.ReadAll(resFile.Body)
		if errbodyFile != nil {
			fmt.Println(errbodyFile)
			return errbodyFile
		}
		resFile.Body.Close()

		var fileContent = string(bodyFile)
		
		for _, elementField := range element.Fields {
			
			var attrValue string
			
			fmt.Println(elementField.Prompt)
			fmt.Scanln(&attrValue) 
			
			fileContent = strings.Replace(fileContent, "_"+elementField.Name+"_", attrValue, -1)
		}
		errfo := ioutil.WriteFile(name+"/"+element.Name, []byte(fileContent), 0644)
		if errfo != nil {
			fmt.Println(errfo)
			return errfo
		}
		fmt.Println("File generated successfully!")
    }
	
	fmt.Println("Project generated successfully!")

	return nil
}

func init() {
	generateCodeCmd.Flags().StringP("type", "t", "", "Type of project")
	generateCodeCmd.Flags().StringP("name", "n", "", "Name of the project")
	generateCodeCmd.Flags().StringP("lang", "l", "", "Programming Language")
	generateCodeCmd.Flags().StringP("bpl", "b", "core", "Name of the boilerplate")
	
	codeCmd.AddCommand(
		generateCodeCmd,
	)

	rootCmd.AddCommand(
		codeCmd,
	)
}