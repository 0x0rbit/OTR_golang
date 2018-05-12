# OTR_GO
This is the implementation of OTR protocol I did along with Jonathan Torchia and Rutwij Kulkarni in the Graduate level ISA 763 - Security Protocol Analysis class. You can see the report if you need more info on the protocol and results etc.

The requirements of the implementation are listed below:

Implement the OTR protocol in Go language. You should test your implementation by running a demo in which the following conversation between Bob and Alice takes place:

Alice: Lights on
Bob: 30 seconds
Alice: Forward drift?
Bob: Yes
Alice: 413 is in
Bob: Houston, Tranquility base here
Alice: The Eagle has landed
Bob: A small step for a student, a giant leap for the group

Note:
3 steps in the OTR:
1.	Exchange secret using D-H
2.	Encrypt a single message
3.	Re-key

=>	Leverage crypto library of golang
•	"crypto/aes"
•	"crypto/cipher"
•	 "crypto/dsa"
•	"crypto/rsa"
•	 "crypto/md5"
•	 "crypto/rand"
•	 "crypto/hmac"

1: Function to generate p and g which are D-H parameters. If this is too challenging choose a known value of p and g
2: Function to generate x of Alice and y of Bob to compute gx mod p, gy mod p and the secret g^xy mod p
3: Picking random number x and y should be done in step 1 and again in step 3 to pick different x’ and y’ for next round
4: Struct for users defining identifier, message, public and private keys, D-H parameters etc
5: You should have a function to create users by assigning values to their parameters defined in the struct
6: You can generate public/private keys using rsa or dsa. Have a function for that
7: Signing g^x and g^y should be done using Alice’s and Bob’s respective private keys
8: For step 2, you can verify by running hmac on the encrypted message using the key MK. You can specify the hashing algorithm used for hmac when creating the hmac object
9: For hashing EK and MK, you can use any hashing algorithm provided by Go such as MD5 or other
10: Encryption and decryption of the message should be done using AES
11: Close attention should be done with synchronization in all steps 1, 2 and 3
