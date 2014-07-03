package main

import(
    "fmt"
    "rtnm/cfg"
    "os"
    "rtnm/roles"
)


func main(){
    if len(os.Args) != 2{
        os.Exit(1)
    }
    cfg_dict := cfg.ReadConfig()
    fmt.Println(cfg_dict)
    if cfg_dict.CnC {
        roles.StartMaster(cfg_dict)
    } else {
        roles.StartProbe(cfg_dict)
    }
}
