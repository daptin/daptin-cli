## Daptin Golang Client


```go
package main

import daptinClient "github.com/daptin/daptin-go-client"
import "log"

func main() {
    clientConnection := daptinClient.NewDaptinClient("http://localhost:6336")
    signUpResponses, err := clientConnection.ExecuteAction("signup", "user_account", map[string]interface{}{
                "email": "test@example.com",
                "name": "test account",
                "password": "test@example.com",
                "passwordConfirm": "test@example.com",
    })
    if err != nil {
        log.Printf("Sign up failed: %v", signUpResponses)
    }
}

```