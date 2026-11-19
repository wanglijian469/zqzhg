package main

import (
 "context"
 "encoding/json"
 "os"
 "path/filepath"
 "testing"
)

// Explicit local acceptance only. Read the developer database through mysqldump,
// then run all migrations and setting changes in a new isolated restore database.
func TestOSSReleaseFixture(t *testing.T){
 if os.Getenv("OSS_RELEASE_FIXTURE")!="1"{t.Skip("explicit local release fixture only")}
 loadEnv();root,_:=filepath.Abs("..");media,_:=filepath.Abs(env("MEDIA_DIR","./data/media"));database:=env("DB_NAME","zqzt_exhibition")
 host,e:=readHostTarget(media);if e!=nil||host.Pending{t.Fatal("active source unavailable")};if host.Current.Database!=""{database=host.Current.Database;media=host.Current.MediaDir}
 source,e:=openExistingDB(database,env("DB_USER","root"),env("DB_PASSWORD","root"));if e!=nil{t.Fatal("source connection failed")};pool,_:=source.DB();defer pool.Close()
 a:=&App{DB:source,Config:Config{Env:"development",MediaDir:media}}
 path:=filepath.Join(t.TempDir(),"source.sql");if e=a.dumpDatabase(context.Background(),path);e!=nil{t.Fatal(e)}
 name:="zqzt_exhibition_restore_"+randomToken()[:16];clone,e:=a.createRestoreDatabase(context.Background(),name,path);if e!=nil{t.Fatal(e)};copyPool,_:=clone.DB();defer copyPool.Close()
 if e=migrate(clone);e!=nil{t.Fatal(e)}
 config,e:=systemConfigDB(clone);if e!=nil{t.Fatal(e)};config.Site.WebURL="http://127.0.0.1:18945";config.Site.APIURL="http://127.0.0.1:18945/api/v1";config.Site.MediaURL="http://127.0.0.1:18945/api/v1/media";config.Version++;raw,_:=json.Marshal(config)
 if e=clone.Model(&SystemSetting{}).Where("setting_key='system_config_v3'").Update("value",string(raw)).Error;e!=nil{t.Fatal(e)}
 receipt:=map[string]any{"database":name,"media_directory":media,"source_database_modified":false,"web_directory":filepath.Join(root,".local/oss-web"),"map_directory":filepath.Join(root,".local/map-preview"),"base":"http://127.0.0.1:18945"}
 output:=filepath.Join(root,".local/oss-candidate/local-fixture.json");raw,_=json.MarshalIndent(receipt,"","  ");if e=os.WriteFile(output,raw,0600);e!=nil{t.Fatal(e)}
}
