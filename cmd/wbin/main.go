package main

import (
	"errors"
	"log"
	"os"

	"github.com/shupkg/wbin/embed"
	"github.com/spf13/pflag"
)

func main() {
	var packer = embed.New()
	pflag.ErrHelp = errors.New("")
	pflag.CommandLine.SortFlags = false
	pflag.StringVarP(&packer.Import, "import", "i", packer.Import, "导入「.Fs, .File」的包名")
	pflag.StringVarP(&packer.Out, "out", "o", packer.Out, "输出文件")
	pflag.StringVar(&packer.Var, "var", packer.Var, "变量命令前缀")
	pflag.StringSliceVar(&packer.Filters, "exclude", packer.Filters, "过滤规则，正则")
	pflag.StringSliceVar(&packer.Files, "file", packer.Files, "要嵌入的文件")
	pflag.BoolVarP(&packer.Force, "force", "f", packer.Force, "覆盖输出文件（如果已存在）")
	pflag.BoolVarP(&packer.Verbose, "verbose", "v", packer.Verbose, "输出过程信息")
	pflag.Parse()

	packer.Files = append(packer.Files, pflag.Args()...)
	if err := packer.Run(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
