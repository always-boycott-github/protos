package meta

import (
	"fmt"
	"os"
)

const banner = `                 ###########
             ###################
          ######             ######
        ####                    #####
      ####                         ####
     ###                            ####
    ###                              ####
   ####                               ####      Protos %s
   ####                               ####      PID: %d
   #######################################      HTTP port: %d    HTTPS port: %d
       ###                         ###          Data dir: %s
       ###############################          Init mode: %t
       ###############################
            ///   ///   ///  ////               https://protos.io
            ///   ///   ///  ////
            ///   ///   ///  ////
            ///   ///   ///  ////
            ///   ///   ///  ////
     //     ///   ///   ///  ////    ///
    ////    ///   ///   ///  ////    ///
     ////  ////   ///   ///   ////  ////
      ///////     ///   ///    ////////
                  ///   ///
                  ///   ///
                  ///   ///
                  ///   ///`

// PrintBanner prints the Protos ascii banner
func PrintBanner() {
	pid := os.Getpid()
	fmt.Println(fmt.Sprintf(
		banner,
		gconfig.Version.String(),
		pid,
		gconfig.HTTPport,
		gconfig.HTTPSport,
		gconfig.WorkDir,
		gconfig.InitMode))
}
