# Used Method: 
   AESGCM
   - AES 256 for encryption
   - SHA 256 for HMAC/authentication
            
# Process:
   Encryption   
    - encrypt the data using aes.  
    - compute hash  
    - append the hash value with the encrypted data.  
   
   Decryption  
    - authenticate the data with the hash  
    - if data is not authentic return  
    - decrypt the data.

# Reading materials
  1. [StackExchange](http://security.stackexchange.com/a/65645)
  2. [GCM](https://en.wikipedia.org/wiki/Galois/Counter_Mode)
  3. [RFC5288 AES_GCM](https://tools.ietf.org/html/rfc5288#page-2)
  4. [RFC4106 GCM ESP](https://tools.ietf.org/html/rfc4106#page-3)
  5. [csrc](http://csrc.nist.gov/groups/ST/toolkit/BCM/documents/proposedmodes/gcm/gcm-spec.pdf)
  6. [AEAD](https://en.wikipedia.org/wiki/Authenticated_encryption)
  7. [Proposal](https://docs.google.com/document/d/1heqCZ1c4lUADQNWJG8RcDE9JNuIL40VePGVeQAjpi1I/edit#bookmark=id.l1ke7mjhe5fy)
  8. [4guysfromrolla](http://www.4guysfromrolla.com/articles/081705-1.aspx)
  
  **Language Specific Implementations are described in the {lang}-impl/README.md**

## Key Generation
Currently the key generation process is simple. we need 32bytes (256bits) of key to encrypt the data.
so if the key length is smaller then 32 we are appending the key in an circular approach until
the key is 32bytes. So a key of `ABCD` will become `ABCDABCDABCDABCDABCDABCDABCDABCD`.

**Proposed Approach of key generation**
To Secure the key we can add some extra layer to the key generation process. This Could
  - find hash of the provided key. possibly - sha256
  - append a salt, salt could be a constant
  - find the hash. possible - md5
  - resize the key to 32 bytes as we are doing now.
      
      
      
##Nonce Generation
Currently we are using the provided key as the nonce.


*What is nonce?*

*Ans: A nonce is a number used once: a nonce should never be reused in a set of messages 
encrypted with the same key. keys are secrets that do not change often
So, you have this vulnerability that if the keys leak, all the secrets leak
so, they augment the secret with a dynamically added secret part that is supposed to be used only one
for extra bit of protection.*

- [NONCE](https://en.wikipedia.org/wiki/Cryptographic_nonce)


 **Proposed Approach of nonce generation**
 
 To Secure the nonce we can add some extra layer to the nonce generation process. This Could
  - generate a random nonce.
  - add this nonce to the encrypted text.
  - while decrypting find the nonce first from the data.
  - use this nonce to the generate decrypted text.

## Notes/Development Guide
  - Encrypted bytes are converted to Base64 String before return.
  - Before Decrypting use Base64 decoder to decode the string to Bytes.
  - Use the test_data data to test the implementation against.
