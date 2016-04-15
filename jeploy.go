package main

import (
    "fmt"
    "os"
    "log"
    "flag"
    "gopkg.in/yaml.v2"
    "io"
    "io/ioutil"
    "github.com/mitchellh/cli"
    "net/http"
    "strconv"
    "archive/zip"
    "path/filepath"
)

const configFile = "jeploy.yml"
const configFilePerm = 644
const currentFolder = "current"

type Config struct {
    Settings struct {
        Project string
        Repo string
    }
    Deploy struct {
        At string
        State string
        Version string
        Old_version string
    }
}

func unzip(src, dest string) error {
    r, err := zip.OpenReader(src)
    if err != nil {
        return err
    }
    defer func() {
        if err := r.Close(); err != nil {
            panic(err)
        }
    }()

    os.MkdirAll(dest, 0755)

    // Closure to address file descriptors issue with all the deferred .Close() methods
    extractAndWriteFile := func(f *zip.File) error {
        rc, err := f.Open()
        if err != nil {
            return err
        }
        defer func() {
            if err := rc.Close(); err != nil {
                panic(err)
            }
        }()

        path := filepath.Join(dest, f.Name)

        if f.FileInfo().IsDir() {
            os.MkdirAll(path, f.Mode())
        } else {
            f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
            if err != nil {
                return err
            }
            defer func() {
                if err := f.Close(); err != nil {
                    panic(err)
                }
            }()

            _, err = io.Copy(f, rc)
            if err != nil {
                return err
            }
        }
        return nil
    }

    for _, f := range r.File {
        err := extractAndWriteFile(f)
        if err != nil {
            return err
        }
    }

    return nil
}

func downloadFile(filepath string, url string) (err error) {

  // Create the file
  out, err := os.Create(filepath)
  if err != nil  {
    return err
  }
  defer out.Close()

  // Get the data
  resp, err := http.Get(url)
  if err != nil {
    return err
  }
  defer resp.Body.Close()

  // Writer the body to file
  _, err = io.Copy(out, resp.Body)
  if err != nil  {
    return err
  }

  return nil
}

func deployVersion(config Config, c *DeployCommand) (err error) {    
    version := c.Version
    folderName := config.Settings.Project + "-" + strconv.Itoa(version)
    fileName := folderName +  ".zip"
    packageUrl := config.Settings.Repo + "/" + config.Settings.Project + "/" + fileName
    if _, err = os.Stat(folderName); os.IsNotExist(err) {
        c.Ui.Output(fmt.Sprintf("Starting download package: %s", packageUrl))
        err = downloadFile(fileName, packageUrl)
        if err != nil {
            return err
        }
        c.Ui.Output("Download finished")

        c.Ui.Output(fmt.Sprintf("Unzipping archive: %s", fileName))
        err = unzip(fileName, folderName)
        if err != nil {
            return err
        }
    } else {
        c.Ui.Output(fmt.Sprintf("Package already exists: %s", fileName))
    }

    if _, err = os.Stat(currentFolder); err == nil {
        err = os.Remove(currentFolder)
        if err != nil {
            return err
        }
    }

    c.Ui.Output(fmt.Sprintf("Setting %s to current", folderName))
    err = os.Symlink(folderName, currentFolder)
    if err != nil {
        return err
    }

    return nil
}

func check(e error) {
    if e != nil {
        panic(e)
    }
}

func readConfig() Config {
    config := Config{}

    configData, err := ioutil.ReadFile(configFile)
    check(err)

    err = yaml.Unmarshal(configData, &config)
    check(err)

    return config
}

func writeConfig(config Config) {
    configData, err := yaml.Marshal(&config)
    check(err)
    
    err = ioutil.WriteFile(configFile, configData, configFilePerm)
    check(err)
}

type DeployCommand struct {
    Version int
    Ui       cli.Ui
}

func (c *DeployCommand) Run(args []string) int {
    cmdFlags := flag.NewFlagSet("deploy", flag.ContinueOnError)
    cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }

    cmdFlags.IntVar(&c.Version, "version", 0, "The version")
    if err := cmdFlags.Parse(args); err != nil {
        return 1
    }
    if c.Version < 1 {
        c.Ui.Output(fmt.Sprintf("Version should be greater than 0. Specified version: %d", c.Version))
        return 1
    }

    c.Ui.Output(fmt.Sprintf("Starting deploy %d version", c.Version))
    c.Ui.Output(fmt.Sprintf("Reading config"))
    config := readConfig()

    err := deployVersion(config, c)
    check(err)

    return 0
}

func (c *DeployCommand) Help() string {
    return "Deploy a specified version of package: jeploy deploy --version 1"
}

func (c *DeployCommand) Synopsis() string {
    return "Deploy a specified version of package"
}

/*func main() {
    config := readConfig()



    config.Deploy.Old_version = "haha"
    writeConfig(config)
}*/

func main() {
    ui := &cli.BasicUi{
        Reader:      os.Stdin,
        Writer:      os.Stdout,
        ErrorWriter: os.Stderr,
    }

    c := cli.NewCLI("jeploy", "0.0.1")
    c.Args = os.Args[1:]
    c.Commands = map[string]cli.CommandFactory{
        "deploy":  func() (cli.Command, error) {
            return &DeployCommand{
                Ui: &cli.ColoredUi{
                    Ui:          ui,
                    OutputColor: cli.UiColorBlue,
                },
            }, nil
        },
        //"rollback": barCommandFactory,
    }

    exitStatus, err := c.Run()
    if err != nil {
        log.Println(err)
    }

    os.Exit(exitStatus)
}
