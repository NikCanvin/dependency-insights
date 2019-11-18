package insights

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"

	"gopkg.in/src-d/go-license-detector.v3/licensedb"
	"gopkg.in/src-d/go-license-detector.v3/licensedb/filer"
)

// ProduceInsights calls the appropriate crawling function for the provided language and then reports on licensing
func ProduceInsights(language string, projectPath string) (*map[string]interface{}, error) {
	_, err := os.Stat(projectPath)

	if err != nil {
		return nil, err
	}

	insightData := make(map[string]interface{})

	// Choose correct walk function depending on the language of the project
	switch language {
	case "nodejs":
		err = nodeWalk(projectPath, insightData)
	case "go":
		err = goWalk(projectPath, insightData)
	default:
		err := errors.New("language not recognised")
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	performLicenseCheck(insightData)

	if err != nil {
		return nil, nil
	}
	return &insightData, nil
}

// nodeWalk recursively walks through installed node packages to map dependencies
func nodeWalk(projectPath string, insightData map[string]interface{}) error {
	if _, err := os.Stat(projectPath + "/node_modules"); err == nil {
		files, err := ioutil.ReadDir(projectPath + "/node_modules")
		if err != nil {
			return err
		}

		for _, file := range files {
			if file.IsDir() {
				path := projectPath + "/node_modules/" + file.Name()
				if _, err := os.Stat(path + "/package.json"); err == nil {
					jsonFile, err := os.Open(path + "/package.json")

					if err != nil {
						return err
					}

					byteValue, err := ioutil.ReadAll(jsonFile)

					if err != nil {
						return err
					}

					jsonFile.Close()

					var result map[string]interface{}
					err = json.Unmarshal([]byte(byteValue), &result)

					if err != nil {
						return err
					}

					newPackageData := make(map[string]interface{})
					transferNodeData(result, newPackageData, path)
					packageID := newPackageData["name"].(string) + "@" + newPackageData["version"].(string)
					if _, ok := insightData[packageID]; !ok {
						insightData[packageID] = newPackageData
					}

					if _, err := os.Stat(path + "/node_modules"); err == nil {
						err := nodeWalk(path, insightData)

						if err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

// goWalk walks through installed go packaged to map dependencies
func goWalk(projectPath string, insightData map[string]interface{}) error {
	// Ensure go packages are installed
	if _, err := os.Stat(projectPath + "/vendor"); err == nil {
		sources, err := ioutil.ReadDir(projectPath + "/vendor")
		if err != nil {
			return err
		}
		// Loop through package sources
		for _, source := range sources {
			if source.IsDir() {
				sourcePath := projectPath + "/vendor/" + source.Name()
				authors, err := ioutil.ReadDir(sourcePath)

				if err != nil {
					return err
				}

				// Look through package authors
				for _, author := range authors {
					completeFlag := false
					authorPath := sourcePath + "/" + author.Name()
					dependencies, err := ioutil.ReadDir(authorPath)

					if err != nil {
						return err
					}

					// Check for non-directory in author path to see if we are at the package level
					for _, dependency := range dependencies {
						if !dependency.IsDir() {
							newPackageData := make(map[string]interface{})
							fullPath := authorPath
							newPackageData["path"] = fullPath
							depName := source.Name() + "/" + author.Name()
							insightData[depName] = newPackageData
							completeFlag = true
						}
					}

					if completeFlag {
						continue
					}

					// Loop through packages
					for _, dependency := range dependencies {
						newPackageData := make(map[string]interface{})
						fullPath := authorPath + "/" + dependency.Name()
						newPackageData["path"] = fullPath
						depName := source.Name() + "/" + author.Name() + "/" + dependency.Name()

						insightData[depName] = newPackageData
					}
				}
			}
		}
	}
	return nil
}

// transferNodeData takes existing package data from a package.json and loads it into a packageData struct
func transferNodeData(packageJSON map[string]interface{}, packageData map[string]interface{}, path string) {
	if str, ok := packageJSON["name"].(string); ok {
		packageData["name"] = str
	}
	if str, ok := packageJSON["version"].(string); ok {
		packageData["version"] = str
	}
	if str, ok := packageJSON["description"].(string); ok {
		packageData["description"] = str
	}
	if str, ok := packageJSON["license"].(string); ok {
		packageData["declaredLicenses"] = str
	} else {
		packageData["declaredLicenses"] = "No Declared License"
	}
	packageData["path"] = path
}

// performLicenseCheck takes an existing map of package data and performs a license check on each package
func performLicenseCheck(insightData map[string]interface{}) {
	for _, depI := range insightData {
		dep := depI.(map[string]interface{})
		filer, err := filer.FromDirectory(dep["path"].(string))

		if err != nil {
			dep["license-analysis"] = err.Error()
			continue
		}

		results, err := licensedb.Detect(filer)

		if err != nil {
			dep["license-analysis"] = err.Error()
		} else {
			dep["license-analysis"] = results
		}
	}
}
