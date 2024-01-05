package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/cockroachdb/pebble"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type Config struct {
	MinioConfig struct {
		Endpoint        string `yaml:"endpoint"`
		AccessKeyID     string `yaml:"accessKeyID"`
		SecretAccessKey string `yaml:"secretAccessKey"`
		BucketName      string `yaml:"bucketName"`
	} `yaml:"minio"`
	APPConfig struct {
		DBPath     string `yaml:"dbPath"`
		UploadPath string `yaml:"uploadPath"`
	} `yaml:"app"`
}

func (c *Config) ReadConfig(configPath string) *Config {
	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	err = yaml.Unmarshal(data, c)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	log.Printf("config:%v\n", c)
	return c
}

// FileMeta 代表文件的元信息
type FileMeta struct {
	SHA        string `json:"sha"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	FilePath   string `json:"file_path"`
	ModifyTime string `json:"modify_time"`
}

// GenerateSHA256 计算文件的 SHA256 散列
func GenerateSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	hash := sha256.New()

	// 将文件内容复制到哈希对象中，io.Copy会在内部处理大文件的分块问题
	if _, err := io.Copy(hash, file); err != nil {
		log.Fatal(err)
	}
	// 计算最终的哈希值

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func main() {

	configFile := "./config.yml"
	// 检查是否有足够的参数
	if len(os.Args) == 2 {
		configFile = os.Args[1]
		log.Println("configFile:", configFile)
	}
	var config Config
	config.ReadConfig(configFile)

	endpoint := config.MinioConfig.Endpoint
	accessKeyID := config.MinioConfig.AccessKeyID
	secretAccessKey := config.MinioConfig.SecretAccessKey
	bucketName := config.MinioConfig.BucketName
	folderPath := config.APPConfig.UploadPath

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: false,
	})
	if err != nil {
		log.Fatalln(err)
	}
	dbFile := config.APPConfig.DBPath
	// 打开（或创建）一个 Pebble 数据库
	db, err := pebble.Open(dbFile, &pebble.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	var uploadFiles []string
	err = filepath.Walk(folderPath, visit(&uploadFiles))
	if err != nil {
		log.Printf("error walking the path %q: %v\n", folderPath, err)
	}

	if len(uploadFiles) > 0 {
		log.Printf("文件总数：%d\n", len(uploadFiles))
	}
	basePath := ""
	for _, filePath := range uploadFiles {
		log.Println(filePath)
		info, err := os.Stat(filePath)
		if err != nil {
			continue
		}
		filename := filepath.Base(filePath)
		if !info.IsDir() {
			log.Println(filename)
			extName := getExtension(filename)
			if ".zip" == extName {
				continue
			}
			// 生成 SHA256 编码
			sha, err := GenerateSHA256(filePath)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println("sha:", sha)
			contentType := GetMimeType(getExtension(filePath))
			log.Println(contentType)
			log.Println(basePath)

			// 检查文件是否已存在
			if value, closer, err := db.Get([]byte(sha)); err == nil {
				closer.Close()
				log.Println("文件已经存在.")
				var fileMetaData FileMeta
				if err := json.Unmarshal(value, &fileMetaData); err != nil {
					log.Printf("Failed to unmarshal JSON data: %v\n", err)
				}
				log.Printf("infoMeta:name:%s,type:%s,path:%s,modfityTime:%s\n", fileMetaData.Name, fileMetaData.Type, fileMetaData.FilePath, fileMetaData.ModifyTime)
			} else {
				basePath = getFilePath(info) + "/"
				modifyTime := getModifyTime(info)
				filename = addModTimeToFilename(filename, modifyTime)
				log.Println("newFileName:", filename)
				uploadInfo, err := minioClient.FPutObject(context.Background(), bucketName, basePath+filename, filePath, minio.PutObjectOptions{ContentType: contentType})
				if err != nil {
					log.Println("uploadInfo.error:", err)
					return
				}
				log.Println("Successfully uploaded " + basePath + " of size " + fmt.Sprintf("%d", uploadInfo.Size))
				log.Printf("%v\n", uploadInfo)
				// 将文件元信息转换为字节以存储在 Pebble
				// 创建文件的元信息
				fileMeta := FileMeta{
					SHA:        sha,
					Name:       filename,
					Type:       contentType,
					FilePath:   basePath,
					ModifyTime: modifyTime,
				}
				metaBytes, err := json.Marshal(fileMeta)
				if err != nil {
					log.Fatal(err)
				}

				// 使用文件 SHA 作为键来存储相关的元信息
				err = db.Set([]byte(fileMeta.SHA), metaBytes, pebble.Sync)
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	}

	if err != nil {
		log.Fatalln(err)
	}

}

func getFilePath(info os.FileInfo) string {
	return info.ModTime().Format("200601")
}

func getModifyTime(info os.FileInfo) string {
	return info.ModTime().Format("20060102150405")
}

func visit(files *[]string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("error accessing path %q: %v\n", path, err)
			return err
		}
		if !info.IsDir() {
			*files = append(*files, path)
		}
		return nil
	}
}

func addModTimeToFilename(filename string, modTime string) string {
	ext := filepath.Ext(filename)                // 获取文件扩展名
	name := filename[0 : len(filename)-len(ext)] // 获取不包含扩展名的文件名
	// 构建新的文件名并返回
	newFilename := fmt.Sprintf("%s-%s%s", name, modTime, ext)
	return newFilename
}

// GetMimeType 通过扩展名返回MIME类型
func GetMimeType(extension string) string {
	// 确保扩展名以"."开头
	if extension[0] != '.' {
		extension = "." + extension
	}
	// 使用mime.TypeByExtension从扩展名获取MIME类型
	mimeType := mime.TypeByExtension(extension)

	return mimeType
}

func getExtension(filePath string) string {
	return filepath.Ext(filePath)
}
