package main

import (
	"bytes"
	"fmt"
	"strings"
	"time"
	"sync"
	"io"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rsa"
	"crypto/rand"
	"crypto/hmac"
	"crypto/sha256"
	"math/big"
	mathr "math/rand"
	"os/exec"
	"log"
	"encoding/base64"
)

/************************************ Diffie-Hellman ******************************************************/

func (u *User) genDHparam(ch_bigint chan *big.Int, ch_int chan int) {    
    out, err := exec.Command("bash", "-c", "openssl gendh -2 1024 | openssl dh -noout -text | sed 's/^[ ^t]*//;s/://g;' | sed -n 3,11p | tr -d '\\n'").Output()
    if err != nil {
        log.Fatal(err)
    }
    output := string(out[:])
    p := new(big.Int)
    p.SetString(output, 16)
    u.dh_p=p
    u.dh_g=2		
    fmt.Println("--> g = ",u.dh_g,"\n--> p =",u.dh_p)
    ch_bigint <- u.dh_p
    ch_int <- u.dh_g
    wg.Done()			
}

func (u *User) genDHkeys (ch_bigint chan *big.Int) {
	secret := new(big.Int)
	public := new(big.Int)
	var err error 

	mathr.Seed(time.Now().UTC().UnixNano())
	
	secret, err = rand.Int(rand.Reader, u.dh_p)
	if err != nil {
        	fmt.Println(err)	
    	}
	u.dh_secret = secret
	public.Exp(big.NewInt(int64(u.dh_g)), u.dh_secret, u.dh_p)
	u.dh_public = public
	fmt.Println("-->",u.identity,"'s picked secret: ",secret)//,"\n-->",u.identity,"'s Public Exponent: ", u.dh_public)
	ch_bigint <- u.dh_public
	wg.Done()	
}

func (u *User) genSharedkey() {
	shared := new(big.Int)

	shared.Exp(u.dh_public_partner, u.dh_secret, u.dh_p)
	u.dh_shared = shared
	fmt.Println("||",u.identity,"||","Generated Shared key: ", u.dh_shared)
	wg.Done()
}

func initializeDHparams (u1 *User, u2 *User){
	channel_bigint := make(chan *big.Int,1)
	channel_int := make(chan int,1)
	wg.Add(1)
	fmt.Println("\n##### DH (p,g) being generated by",u1.identity,"#####")	
	go u1.genDHparam(channel_bigint, channel_int)
	wg.Wait()
	u2.dh_p = <-channel_bigint
	u2.dh_g = <-channel_int
	fmt.Println("##### DH (p,g) successfully shared with ",u2.identity,"#####")
}

func initializeDH (u1 *User, u2 *User) {
	channel_digest := make(chan []byte,1)
	channel_sig := make(chan []byte,1)
	channel_bool := make(chan bool,1)
	channel_bigint := make(chan *big.Int,1)
	
	wg.Add(3)
	fmt.Println("\n\n##### DH (Public,Secret) exponents being generated by",u1.identity,"#####")
	go u1.genDHkeys(channel_bigint)
	go u1.genSHA256(<-channel_bigint, channel_digest, channel_bigint)
	go u1.genRSAsig(<-channel_digest, channel_sig)
	wg.Wait()
	fmt.Println("\n\n>>>>> Sharing",u1.identity,"'s public exponent with ",u2.identity,"<<<<<")	
	wg.Add(2)
	go u2.genSHA256(<-channel_bigint, channel_digest, channel_bigint)
	go u2.verifyRSAsig(<-channel_digest,<-channel_sig, channel_bool)
	wg.Wait()
	if (<-channel_bool){
		fmt.Println("\n*** RSA Signature verification successful. Accepting",u1.identity,"'s public exponent ***")
		u2.dh_public_partner = <-channel_bigint
	} else {
		fmt.Println("\n*** RSA Signature verification successful. Rejecting",u1.identity,"'s public exponent ***")
	}
}

/***********************************************************************************************/

/************************************ RSA ******************************************************/

func (u *User) genRSAkeys (bits int, channel chan *rsa.PublicKey) {
	private_key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
        	fmt.Println(err)	
    	}
	fmt.Println("*** Successfully generated RSA key pair for", u.identity," ***")
	public_key := private_key.PublicKey
	u.rsaKey = private_key
	u.rsaPubKey = &public_key
	channel <- u.rsaPubKey 
	wg.Done()
}

func (u *User) genRSAsig (digest []byte, channel chan []byte) {
	signature, err := rsa.SignPKCS1v15(rand.Reader, u.rsaKey, crypto.SHA256, digest)
	if err != nil {
        	fmt.Println(err)	
    	}
	channel <- signature
	fmt.Printf("\n*** Generated RSA Signature ***")
	wg.Done()
}

func (u *User) verifyRSAsig (digest []byte, signature []byte, channel chan bool) {
	err := rsa.VerifyPKCS1v15(u.rsaPubKey_partner, crypto.SHA256, digest, signature)
	if err != nil {
        	fmt.Println(err)
		channel <- false	
    	}
	channel <- true
	wg.Done()
}

func initializeRSA(u1 *User, u2 *User){
	channel_rsaPubKey := make(chan *rsa.PublicKey, 2)
	fmt.Println("##### RSA Initialization being done for users #####")
	wg.Add(1)
	go u1.genRSAkeys(2048,channel_rsaPubKey)
	wg.Wait()
	wg.Add(1)
	go u2.genRSAkeys(2048,channel_rsaPubKey)
	wg.Wait()
	u2.rsaPubKey_partner = <-channel_rsaPubKey
        u1.rsaPubKey_partner = <-channel_rsaPubKey
	fmt.Println("*** RSA Public key exchange successfully completed between",u1.identity,"<<--->>",u2.identity,"***")
}


/***********************************************************************************************/

/************************************ SHA256 Digest ***********************************************/

func (u *User) genSHA256 (b *big.Int, channel chan []byte, ch_int chan *big.Int) {
	 
         digest := sha256.Sum256([]byte(b.String()))
	 fmt.Printf("*** Generated SHA256 Digest ***")
	 channel <- digest[:]
	 ch_int <- b
	 wg.Done()
}

/*****************************************************************************************/

/************************************ HMAC ***********************************************/

func (u *User) genMACkey (message []byte, channel chan []byte) {
	fmt.Printf("\n*** Generated HMAC ***")
	mac := hmac.New(sha256.New, u.mk)
	mac.Write(message)
	messageMAC := mac.Sum(nil)
	channel <- messageMAC[:]
	wg.Done()
}

func (u *User) checkMACkey(message []byte, messageMAC []byte, channel chan bool) {
	fmt.Printf("\n*** Checking HMAC integrity ***")	
	mac := hmac.New(sha256.New, u.mk)
	mac.Write(message)
	expectedMAC := mac.Sum(nil)
	channel <- hmac.Equal(messageMAC, expectedMAC)
	wg.Done()
}

func (u *User) genMACmsg (message string, channel chan []byte, channel_string chan string) {
	fmt.Printf("\n*** Generated HMAC ***")
	mac := hmac.New(sha256.New, u.mk)
	mac.Write([]byte(message))
	messageMAC := mac.Sum(nil)
	channel <- messageMAC[:]
	channel_string <- message
	wg.Done()
}

func (u *User) checkMACmsg(message string, messageMAC []byte, channel chan bool, channel_string chan string) {
	fmt.Printf("\n*** Checking HMAC integrity ***")	
	mac := hmac.New(sha256.New, u.mk)
	mac.Write([]byte(message))
	expectedMAC := mac.Sum(nil)
	channel <- hmac.Equal(messageMAC, expectedMAC)
	channel_string <- message
	wg.Done()
}
/*****************************************************************************************/

/************************************ AES ************************************************/

func addBase64Padding(value string) string {
    m := len(value) % 4
    if m != 0 {
        value += strings.Repeat("=", 4-m)
    }

    return value
}

func removeBase64Padding(value string) string {
    return strings.Replace(value, "=", "", -1)
}

func Pad(src []byte) []byte {
    padding := aes.BlockSize - len(src)%aes.BlockSize
    padtext := bytes.Repeat([]byte{byte(padding)}, padding)
    return append(src, padtext...)
}

func Unpad(src []byte) ([]byte) {
    length := len(src)
    unpadding := int(src[length-1])

    if unpadding > length {
        fmt.Println("Unpadding Error !!")
    }

    return src[:(length - unpadding)]
}

func (u *User) encrypt(text string, channel chan string) {
    fmt.Println("++++++++++++++++ Encrypting using AES +++++++++++++++")	
    block, err := aes.NewCipher(u.ek)
    if err != nil {
        fmt.Println(err)
    }

    msg := Pad([]byte(text))
    ciphertext := make([]byte, aes.BlockSize+len(msg))
    iv := ciphertext[:aes.BlockSize]
    if _, err := io.ReadFull(rand.Reader, iv); err != nil {
        fmt.Println(err)
    }

    cfb := cipher.NewCFBEncrypter(block, iv)
    cfb.XORKeyStream(ciphertext[aes.BlockSize:], []byte(msg))
    finalMsg := removeBase64Padding(base64.URLEncoding.EncodeToString(ciphertext))
    channel <- finalMsg
    fmt.Println("|| Encrypted Message:", finalMsg,"||")
    fmt.Println("++++++++++++++++++++++++++++++++++++++++++++++++")	
    wg.Done()	
}

func (u *User) decrypt(text string) {
    fmt.Println("+++++++++++ Decrypting using AES ++++++++++++")
    block, err := aes.NewCipher(u.ek)
    if err != nil {
        fmt.Println(err)
    }

    decodedMsg, err := base64.URLEncoding.DecodeString(addBase64Padding(text))
    if err != nil {
        fmt.Println(err)
    }

    if (len(decodedMsg) % aes.BlockSize) != 0 {
        fmt.Println("Blocksize must be multiple of decoded message length")
    }

    iv := decodedMsg[:aes.BlockSize]
    msg := decodedMsg[aes.BlockSize:]

    cfb := cipher.NewCFBDecrypter(block, iv)
    cfb.XORKeyStream(msg, msg)

    unpadMsg := Unpad(msg)

    fmt.Println("|| Message Received by",u.identity,": \"", string(unpadMsg[:]),"\" ||")
    fmt.Println("++++++++++++++++++++++++++++++++++++++++++++++++")		
    wg.Done()	
}
/***********************************************************************************************/

/************************************ OTR ***********************************************/

func (u *User) genEKMK () {
	ek := sha256.Sum256([]byte(u.dh_shared.String()))
	mk := sha256.Sum256(ek[:])
	u.ek = ek[:]
	u.mk = mk[:]
	wg.Done()
}

func genKeys(u1 *User, u2 *User){
	fmt.Println("##### Generating DH shared secret #####")	
	wg.Add(2)
	go u1.genSharedkey()	
	go u2.genSharedkey()
	wg.Wait()
	
	fmt.Println("##### Generating EK and MK #####")	
	wg.Add(2)
	go u1.genEKMK()	
	go u2.genEKMK()
	wg.Wait()
}

func reKeying (u1 *User, u2 *User) {
	channel_bigint := make(chan *big.Int,1)
	channel_digest := make(chan []byte,1)
	channel_mac := make(chan []byte,1)
	channel_bool := make(chan bool,1)
	wg.Add(3)
	fmt.Println("##### DH (Public,Secret) exponents being generated by",u1.identity,"#####")
	go u1.genDHkeys(channel_bigint)
	go u1.genSHA256(<-channel_bigint, channel_digest, channel_bigint)
	go u1.genMACkey(<-channel_digest, channel_mac)
	wg.Wait()
	fmt.Println("\n\n>>>>> Sharing",u1.identity,"'s public exponent with ",u2.identity,"<<<<<")	
	wg.Add(2)
	go u2.genSHA256(<-channel_bigint, channel_digest, channel_bigint)
	go u1.checkMACkey(<-channel_digest, <-channel_mac, channel_bool)
	wg.Wait()
	if (<-channel_bool){
		fmt.Println("\n*** MAC verification successful. Accepting",u1.identity,"'s public exponent ***\n\n")
		u2.dh_public_partner = <-channel_bigint
	} else {
		fmt.Println("\n*** MAC verification failed. Rejecting",u1.identity,"'s public exponent ***\n\n")
	}
}

func reKey (u1 *User, u2 *User) {
	fmt.Println("\n---------------------------------------------------------------------")
	fmt.Println("<<<<<<<<<<<<< RE-KEYING REQUEST BY : ",u1.identity,">>>>>>>>>>>>>")
	fmt.Println("---------------------------------------------------------------------")
	reKeying(u1, u2)
	reKeying(u2, u1)
	genKeys(u1, u2)
	fmt.Println("---------------------------------------------------------------------")
}

func commMsg(u1 *User, u2 *User, i int) {
	fmt.Println("\n\n*********************************************************************")
	fmt.Println("                        MESSAGE CONSOLE                              ")
	fmt.Println("*********************************************************************")
	fmt.Println("\n----------------------------------------")
	fmt.Println(">>->>->>->> SENDING MESSAGE >>->>->>->>")
	fmt.Println("----------------------------------------")
	fmt.Println("||",u1.identity,": \"",u1.messages[i],"\" ||\n")
	channel_string := make(chan string, 1)
	channel_mac := make(chan []byte, 1)
	channel_bool := make(chan bool,1)
	wg.Add(2)
	go u1.encrypt(u1.messages[i],channel_string)
	go u1.genMACmsg(<-channel_string, channel_mac, channel_string)
	wg.Wait()
	fmt.Println("\n\n----------------------------------------")
	fmt.Println("<<-<<-<<-<< RECEIVING MESSAGE <<-<<-<<-<<")
	fmt.Println("----------------------------------------")	
	wg.Add(1)
	go u2.checkMACmsg(<-channel_string,<-channel_mac,channel_bool,channel_string)
	wg.Wait()
	if (<-channel_bool){
		fmt.Println("\n*** MAC verification successful. Accepting received message ***\n\n")
		wg.Add(1)
		go u2.decrypt(<-channel_string)
		wg.Wait()
	} else {
		fmt.Println("\n*** MAC verification failed. Rejecting received message ***\n\n")
	}
	fmt.Println("*********************************************************************")
}
	
/***********************************************************************************************/

type User struct {
	identity string   // User's Name
	messages []string // User's Messages to send
	rsaPubKey *rsa.PublicKey // RSA Public key of user
	rsaPubKey_partner *rsa.PublicKey // RSA Public key of partner
	rsaKey *rsa.PrivateKey // RSA private key of user
	dh_p *big.Int // Value of DH prime 
	dh_g int // Value of DH group
	dh_secret *big.Int // Value of user's selected secret DH exponent e.g. x
	dh_public *big.Int // Value of user's public key i.e. (g)^x mod p
	dh_public_partner *big.Int // Value of partner's public key i.e. (g)^y mod p
	dh_shared *big.Int // Value of shared key (g)^xy mod p
	ek []byte // Encryption Key
	mk []byte // Key for HMAC
}
	 
var wg sync.WaitGroup

func main() {
	mathr.Seed(time.Now().UTC().UnixNano())
	
	u1 := User{identity: "Alice", messages: []string {"Lights on","Forward drift?","413 is in","The Eagle has landed"}}
	u2 := User{identity: " Bob ", messages: []string {"30 seconds", "Yes", "Houston, Tranquility base here", "A small step for a student, a giant leap for the group"}}
	
	initializeRSA(&u1,&u2) 

	initializeDHparams(&u1,&u2)

	initializeDH(&u1,&u2)
	initializeDH(&u2,&u1)
	genKeys(&u1, &u2)
	j:= 0	
	for i:=0;i<8; i++ {
		if (i%2==0) {
			commMsg(&u1,&u2,j)
			reKey(&u2,&u1)
		} else {
			commMsg(&u2,&u1,j)
			reKey(&u2,&u1)
			j++
		}
	}	
}