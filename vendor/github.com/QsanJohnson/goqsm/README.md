
# goqsm
Go http client to manage Qsan QSM models.

## Install
```
go get github.com/QsanJohnson/goqsm
```

## Usage
Here is an sample code.
```
import (
	"github.com/QsanJohnson/goqsm"
	"fmt"
	"context"
)
	
ctx := context.Background()

client := goqsm.NewClient("192.xxx.xxx.xxx")
systemAPI := goqsm.NewSystem(client)
res, err := systemAPI.GetAbout(ctx)
if err == nil {
	fmt.Printf("%+v\n", res);
}

authClient, err := client.GetAuthClient(ctx, "admin", "1234")
volumeAPI := goqsm.NewVolume(authClient)
vols, err := volumeAPI.ListVolumes(ctx, "", "")
if err == nil {
	fmt.Printf("%+v\n", vols);
}
```

## Debugging
Add flag.Parse() at the begining in main(),
then execute go run with "-v=4 -alsologtostderr" arguments.
```
go run test.go -v=4 -alsologtostderr
```


## Testing

You have to create a test.conf file for integration test. The following is an example,
```
QSM_IP = 192.xxx.xxx.xxx
QSM_USERNAME = admin
QSM_PASSWORD = 1234
TEST_SC_ID = xxxxxx
```

Then run integration test
```
go test
```

Or run integration test with log level
```
export GOQSM_LOG_LEVEL=4
go test
```