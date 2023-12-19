# Go program to remove duplicate files and Upload to MinIO
Introducing a robust Go (Golang) application designed to effectively detect and eliminate duplicate files prior to uploading them onto a MinIO server, a popular object storage solution. This program streamlines storage management and optimizes space utilization by ensuring that only unique files are retained and transferred to the MinIO bucket.

```yaml
app:
  dbPath: "D:/data/pebble/pebbleDb"
  uploadPath: "D:/photo"
minio:
  endpoint: "127.0.0.1:9000"
  accessKeyID: "6ii0AyBgDufNA2XTai0N"
  secretAccessKey: "X8QgIeo6yyV9aoVojaVR2ZFCkAnWfRiJ8oPTy2ej"
  bucketName: "my-bucket"
```
### Installing
Follow these steps to get your development environment running:
1. Clone the repository to your local machine:
```bash
git clone https://github.com/markusleevip/go-minio-upload
cd go-minio-upload
go mod tidy
```
2. Build the project
```bash
go build cmd/upload.go
cp cmd/upload.exe ./
```
3. Run the project
```bash
upload.exe
```





