package embed

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/shupkg/wbin"
)

func New() *Packer {
	return (&Packer{}).Default()
}

type Packer struct {
	Files   []string //要打包的文件
	Out     string   //输出文件
	Force   bool     //覆盖输出文件
	Import  string   //导入「.Fs, .File」的包名
	Var     string   //文件系统的变量名
	Filters []string //排除规则，正则
	Verbose bool     //输出详细过程
}

//执行
func (p *Packer) Default() *Packer {
	p.Import = "github.com/shupkg/wbin"
	p.Filters = []string{`.*\.go$`, `\.DS_Store$`}
	return p
}

//执行
func (p *Packer) Run() error {
	fs, err := p.Walk()
	if err != nil {
		return fmt.Errorf("读取文件出错: %w", err)
	}

	if strings.HasSuffix(p.Out, ".go") {
		//写入文件系统
		if p.Verbose {
			log.Println("写入文件系统")
		}
		p.Out, _ = filepath.Abs(p.Out)
		if p.Verbose {
			log.Println("输出文件:", p.Out)
		}
		if err := p.PackFs(fs); err != nil {
			if os.IsExist(err) {
				fmt.Println("写入的文件已存在，如需覆盖请增加 --force/-f 参数")
				os.Exit(1)
			}
			return err
		}
	} else {
		//写入单文件
		if p.Verbose {
			log.Println("写入文件")
		}
		for truePath, file := range fs {
			if file.IsDir() {
				continue
			}

			var out string
			if p.Out != "" {
				out = filepath.Join(p.Out, file.Path[1:]+".go")
			} else {
				out = truePath + ".go"
			}

			out, _ = filepath.Abs(out)
			if p.Verbose {
				log.Println("输出文件:", out)
			}

			if err := p.PackFile(file, out); err != nil {
				if os.IsExist(err) {
					fmt.Println("写入的文件已存在，如需覆盖请增加 --force/-f 参数")
					os.Exit(1)
				}
				return err
			}
		}
	}
	if p.Verbose {
		log.Println("完成")
	}
	return nil
}

//解析文件
func (p *Packer) Walk() (wbin.Fs, error) {
	if p.Verbose {
		log.Println("解析文件")
	}
	var result = map[string]*wbin.File{}
	for _, file := range p.Files {
		file, _ = filepath.Abs(file)
		err := filepath.Walk(file, func(truePath string, info os.FileInfo, err error) error {
			var path = truePath
			for _, s := range p.Filters {
				if regexp.MustCompile(s).MatchString(path) {
					if p.Verbose {
						log.Println(" ", truePath, " -> 跳过")
					}
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			if file == truePath {
				if !info.IsDir() {
					path = "/" + filepath.Base(truePath)
				} else {
					path = "/"
				}
			} else {
				path, _ = filepath.Rel(file, truePath)
				if !strings.HasPrefix(path, "/") {
					path = "/" + path
				}
			}

			if p.Verbose {
				log.Println(" ", truePath, " -> ", path)
			}

			f := &wbin.File{
				Path:        path,
				FileName:    info.Name(),
				FileSize:    info.Size(),
				FileModTime: info.ModTime().Unix(),
				FileIsDir:   info.IsDir(),
			}
			if !f.FileIsDir && info.Size() > 0 {
				data, err := ioutil.ReadFile(truePath)
				if err != nil {
					return err
				}
				f.Data, err = p.encode(data)
				if err != nil {
					return err
				}
			}

			result[truePath] = f
			return nil
		})
		if err != nil {
			return result, err
		}
	}

	//删除空文件夹
	for key, file := range result {
		if file.IsDir() {
			var hasChild bool
			for _, f := range result {
				if strings.HasPrefix(f.Path, file.Path) && !f.IsDir() {
					hasChild = true
					break
				}
			}
			if !hasChild {
				delete(result, key)
			}
		}
	}

	if p.Verbose {
		log.Printf("解析文件完成，共有 %d 个文件（夹）\n", len(result))
	}
	return result, nil
}

//写入文件系统
func (p *Packer) PackFs(fs wbin.Fs) error {
	var w = &bytes.Buffer{}
	var keys []string
	for s := range fs {
		keys = append(keys, s)
	}
	sort.Strings(keys)

	p.appendHead(w, p.getDIR(p.Out))
	fmt.Fprintln(w)

	varName := p.Var
	if varName == "" {
		varName = p.getName(p.Out, true)
	}
	fmt.Fprintf(w, "var %s = %s.Fs{\n", varName, filepath.Base(p.Import))
	for _, key := range keys {
		file := fs[key]
		fmt.Fprintf(w, "%q: ", file.Path)
		p.appendFileBody(w, file)
		fmt.Fprintln(w, ",")
	}
	fmt.Fprint(w, "}")

	return p.WriteFile(p.Out, w.Bytes())
}

//写入单文件
func (p *Packer) PackFile(file *wbin.File, writeTo string) error {
	var w = &bytes.Buffer{}
	p.appendHead(w, p.getDIR(writeTo))
	fmt.Fprintln(w)

	fmt.Fprintf(w, "var %s%s = &%s.File",
		p.Var,
		p.getName(file.FileName, false),
		filepath.Base(p.Import),
	)
	p.appendFileBody(w, file)
	return p.WriteFile(writeTo, w.Bytes())
}

//写入文件
func (p *Packer) WriteFile(filename string, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	path, _ := filepath.Abs(filename)
	if path == "" {
		return fmt.Errorf("路径无效: %s", filename)
	}
	i, err := os.Stat(path)
	if !p.Force {
		if i != nil {
			return os.ErrExist
		}
		if !os.IsNotExist(err) {
			return err
		}
	}
	if i != nil && i.IsDir() {
		return fmt.Errorf("文件路径是个目录: %s", filename)
	}

	if i == nil {
		os.MkdirAll(filepath.Dir(path), 0755)
	}

	return ioutil.WriteFile(path, goFmt(data), 0644)
}

//写入文件头
func (p *Packer) appendHead(w io.Writer, distPackage string) {
	fmt.Fprintf(w, "package %s\n", distPackage)
	if p.Import != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "import (")
		fmt.Fprintf(w, "%q\n", p.Import)
		fmt.Fprintln(w, ")")
	}
}

//写入文件内容
func (p *Packer) appendFileBody(b io.Writer, file *wbin.File) {
	fmt.Fprintln(b, "{")
	fmt.Fprintf(b, "FileName:%q,\n", file.FileName)
	fmt.Fprintf(b, "FileModTime:%d,\n", file.FileModTime)
	if !file.FileIsDir {
		if file.FileSize > 0 {
			fmt.Fprintf(b, "FileSize:%d,\n", file.FileSize)
			fmt.Fprintf(b, "Data:`%s`,\n", file.Data)
			fmt.Fprintln(b)
		}
	} else {
		fmt.Fprintf(b, "FileIsDir:%t,\n", file.FileIsDir)
	}
	fmt.Fprint(b, "}")
}

//编码文件
func (p *Packer) encode(data []byte) (string, error) {
	zip := func(v []byte) ([]byte, error) {
		var buf bytes.Buffer
		gw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
		defer gw.Close()

		if _, err := gw.Write(v); err != nil {
			return nil, err
		}

		if err := gw.Close(); err != nil {
			return nil, err
		}

		return buf.Bytes(), nil
	}

	var err error
	if data, err = zip(data); err != nil {
		return "", err
	}

	b64 := base64.StdEncoding.EncodeToString(data)
	var b = strings.NewReader(b64)
	var res bytes.Buffer
	chunk := make([]byte, 80)
	res.WriteByte('\n')
	for n, _ := b.Read(chunk); n > 0; n, _ = b.Read(chunk) {
		res.Write(chunk[0:n])
		res.WriteByte('\n')
	}

	return res.String(), nil
}

//从文件名生成变量名
func (p *Packer) getName(path string, trimExt bool) string {
	name, _ := filepath.Abs(path)
	name = filepath.Base(name)
	if trimExt {
		ext := filepath.Ext(name)
		if ext != "" {
			name = strings.TrimSuffix(name, ext)
		}
	}
	name = regexp.MustCompile(`[^a-zA-Z]+`).ReplaceAllString(name, "_")
	name = regexp.MustCompile(`_[a-zA-Z]`).ReplaceAllStringFunc(name, func(s string) string {
		return strings.ToUpper(s[1:])
	})
	name = strings.ToUpper(name[:1]) + name[1:]
	return name
}

//从文件路径生成包名
func (p *Packer) getDIR(path string) string {
	path, _ = filepath.Abs(path)
	return filepath.Base(filepath.Dir(path))
}
