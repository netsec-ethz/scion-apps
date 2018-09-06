package utils;

import (
    "fmt"
    "os"
    "encoding/json"
    "crypto/rand"
    "io/ioutil"
    "encoding/hex"
    "bufio"
    "bytes"
    "log"
    "path/filepath"

    "golang.org/x/crypto/ed25519"

    "github.com/scionproto/scion/go/lib/snet"

    "roughtime.googlesource.com/go/config"
    "roughtime.googlesource.com/go/protocol"
)

func generateKeypair(privateKeyFile string)(rootPrivate ed25519.PrivateKey, rootPublic ed25519.PublicKey, err error){
    rootPublic, rootPrivate, err = ed25519.GenerateKey(rand.Reader)
    if err != nil {
        err=fmt.Errorf("Error creating server keypair %v", err)
        return
    }

    pkFile, err := os.Create(privateKeyFile)
    if err != nil {
        err=fmt.Errorf("Error creating private key file %v", err)
        return
    }
    defer pkFile.Close()

    w := bufio.NewWriter(pkFile)
    _, err = fmt.Fprintf(w, "%x", rootPrivate)
    if err != nil{
        err=fmt.Errorf("Error writing private key to file %v", err)
        return
    }
    w.Flush()

    return
}

func createConfigFile(pubKey ed25519.PublicKey, address *snet.Addr, serverName, configFile string)(error){
    configInformation := config.Server{
        Name:          serverName,
        PublicKeyType: "ed25519",   // For now this is fixed
        PublicKey:     pubKey,
        Addresses: []config.ServerAddress{
            config.ServerAddress{
                Protocol: "udp4",   // For now this is fixed
                Address:  address.String(),
            },
        },
    }

    jsonBytes, err := json.MarshalIndent(configInformation, "", "  ")
    if err != nil {
        return err
    }

    file, err := os.Create(configFile)
    if err != nil {
        return fmt.Errorf("Error creating config file %v", err)
    }
    defer file.Close()

    _, err = file.Write(jsonBytes)
    if err != nil {
        return fmt.Errorf("Error writing configuration to file %v", err)
    }

    file.Sync()

    return nil
}

func GenerateServerConfiguration(address, privateKeyFile, configFile, serverName string)(error){
    serverAddr, err := snet.AddrFromString(address)
    if err!= nil{
        return fmt.Errorf("Invalid scion address! %v", err)
    }

    _, public, err:=generateKeypair(privateKeyFile)
    if err!=nil{
        return err
    }

    err = createConfigFile(public, serverAddr, serverName, configFile)
    if err!=nil{
        return err
    }

    return nil 
}

func LoadServerConfiguration(configurationPath string)(*config.Server, error){
    fileData, err := ioutil.ReadFile(configurationPath)
    if err != nil {
        return nil, fmt.Errorf("Error opening configuration file %v",err)
    }

    var serverConfig config.Server
    if err := json.Unmarshal(fileData, &serverConfig); err != nil {
        return nil, fmt.Errorf("Error parsing configuration file %v", err)
    }

    return &serverConfig, nil
}

func ReadPrivateKey(privateKeyFile string)(ed25519.PrivateKey, error){
    privateKeyHex, err := ioutil.ReadFile(privateKeyFile)
    if err != nil {
        return nil, fmt.Errorf("Cannot open private key file %v", err)
    }

    privateKey, err := hex.DecodeString(string(bytes.TrimSpace(privateKeyHex)))
    if err != nil {
        return nil, fmt.Errorf("Cannot parse private key %v", err)
    }

    return privateKey, nil
}

func GetServerAddr(server *config.Server) (*snet.Addr, error) {
    for _, addr := range server.Addresses {
        if addr.Protocol != "udp4" {
            continue
        }

        return snet.AddrFromString(addr.Address)
    }

    return nil, nil
}

func LoadServersConfigurationList(configurationPath string)(servers []config.Server,err error){
    fileData, err := ioutil.ReadFile(configurationPath)
    if err != nil {
        return nil, fmt.Errorf("Error opening configuration file %v",err)
    }

    var serversJSON config.ServersJSON
    if err = json.Unmarshal(fileData, &serversJSON); err != nil {
        return nil, fmt.Errorf("Error parsing configuration file %v", err)
    }

    seenNames := make(map[string]bool)

    for _, candidate := range serversJSON.Servers {
        if _, ok := seenNames[candidate.Name]; ok {
            return nil, fmt.Errorf("client: duplicate server name %q", candidate.Name)
        }
        seenNames[candidate.Name] = true

        if candidate.PublicKeyType != "ed25519" {
            log.Printf("Skipping unsupported public key type: %s", candidate.PublicKeyType)
            continue
        }

        serverAddr, err := GetServerAddr(&candidate)
        if err != nil {
            return nil, fmt.Errorf("client: server %q lists invalid SCION address: %s", candidate.Name, err)
        }

        if serverAddr == nil {
            log.Printf("Skipping unsupported protocol for server %s", candidate.Name)
            continue
        }

        servers = append(servers, candidate)
    }

    if len(servers) == 0 {
        return nil, fmt.Errorf("client: no usable servers found")
    }

    return servers, nil
}

func LoadChain(chainFilePath string)(chain *config.Chain, err error){
    if _, err := os.Stat(chainFilePath); os.IsNotExist(err) {
        // File doesn't exist, returning empty chain
        return new(config.Chain), nil
    }

    fileData, err := ioutil.ReadFile(chainFilePath)
    if err != nil {
        return nil, fmt.Errorf("Error opening chain file %v", err)
    }

    chain = new(config.Chain)
    if err := json.Unmarshal(fileData, chain); err != nil {
        return nil, err
    }

    for i, link := range chain.Links {
        if link.PublicKeyType != "ed25519" {
            return nil, fmt.Errorf("client: link #%d in chain file has unknown public key type %q", i, link.PublicKeyType)
        }

        if l := len(link.PublicKey); l != ed25519.PublicKeySize {
            return nil, fmt.Errorf("client: link #%d in chain file has bad public key of length %d", i, l)
        }

        if l := len(link.NonceOrBlind); l != protocol.NonceSize {
            return nil, fmt.Errorf("client: link #%d in chain file has bad nonce/blind of length %d", i, l)
        }

        var nonce [protocol.NonceSize]byte
        if i == 0 {
            copy(nonce[:], link.NonceOrBlind[:])
        } else {
            nonce = protocol.CalculateChainNonce(chain.Links[i-1].Reply, link.NonceOrBlind[:])
        }

        if _, _, err := protocol.VerifyReply(link.Reply, link.PublicKey, nonce); err != nil {
            return nil, fmt.Errorf("client: failed to verify link #%d in chain file", i)
        }
    }

    return chain, nil
}

func trimChain(chain *config.Chain, n int) {
    if n <= 0 {
        chain.Links = nil
        return
    }

    if len(chain.Links) <= n {
        return
    }

    numToTrim := len(chain.Links) - n
    for i := 0; i < numToTrim; i++ {
        // The NonceOrBlind of the first element is special because
        // it's an nonce. All the others are blinds and are combined
        // with the previous reply to make the nonce. That's not
        // possible for the first element because there is no previous
        // reply. Therefore, when removing the first element the blind
        // of the next element needs to be converted to an nonce.
        nonce := protocol.CalculateChainNonce(chain.Links[0].Reply, chain.Links[1].NonceOrBlind[:])
        chain.Links[1].NonceOrBlind = nonce[:]
        chain.Links = chain.Links[1:]
    }
}

func SaveChain(chainFile string, chain *config.Chain, maxLength int)(error){
    trimChain(chain, maxLength)
    chainBytes, err := json.MarshalIndent(chain, "", "  ")
    if err != nil {
        return err
    }

    tempFile, err := ioutil.TempFile(filepath.Dir(chainFile), filepath.Base(chainFile))
    if err != nil {
        return err
    }
    defer tempFile.Close()

    if _, err := tempFile.Write(chainBytes); err != nil {
        return err
    }

    if err := os.Rename(tempFile.Name(), chainFile); err != nil {
        return err
    }

    return nil
}