#!/usr/bin/env node
// ECIES-encrypts a PredictionMarket bet payload for the TEE, byte-for-byte
// compatible with go-ethereum's crypto/ecies (ECIES_AES128_SHA256, s1=s2=nil),
// as used by tee-node's pkg/utils/crypto.go Encrypt().
//
// Wire format (matches ecies.Encrypt's output):
//   ephemPubKey(65, uncompressed 0x04||X||Y) || iv(16) || ciphertext || mac(32)
//
// Plaintext: abi.encode(bool isUp, uint256 amount) — two 32-byte words.
//
// Usage: node encrypt_bet.mjs <true|false> <amount>

import { createECDH, createHash, createHmac, createCipheriv, randomBytes } from 'node:crypto';

// TEE public key, from https://flare-tee.idolpulse.com/info -> machineData.publicKey
// Verified against on-chain PredictionMarket.teeAddress() == 0x04BA567472B68fA3bdB04359E0Dd838b11378E55
// via keccak256(x||y)[-20:] before use.
const TEE_PUBKEY_X = '2f195465b8219db4f61083e04116851bcdfff87d2abc593ca11994dee5930cac';
const TEE_PUBKEY_Y = '598b70fcafeb55bf40aef7bed18b206c5eff6b4593278163d7182fd2b16752ad';

function abiEncodeBoolUint256(isUp, amount) {
  const isUpSlot = Buffer.alloc(32);
  isUpSlot[31] = isUp ? 1 : 0;

  const amountSlot = Buffer.alloc(32);
  let hex = amount.toString(16);
  if (hex.length % 2 !== 0) hex = '0' + hex;
  const amountBuf = Buffer.from(hex, 'hex');
  if (amountBuf.length > 32) throw new Error('amount overflows uint256');
  amountBuf.copy(amountSlot, 32 - amountBuf.length);

  return Buffer.concat([isUpSlot, amountSlot]);
}

function eciesEncrypt(plaintext, pubKeyXHex, pubKeyYHex) {
  const recipientPubKey = Buffer.concat([
    Buffer.from([0x04]),
    Buffer.from(pubKeyXHex, 'hex'),
    Buffer.from(pubKeyYHex, 'hex'),
  ]);
  if (recipientPubKey.length !== 65) throw new Error('recipient pubkey must be 65 bytes uncompressed');

  // 1. Ephemeral keypair + ECDH shared secret (z = X coord of ephemPriv * recipientPub)
  const ecdh = createECDH('secp256k1');
  ecdh.generateKeys();
  const ephemPubKey = ecdh.getPublicKey(null, 'uncompressed'); // 65 bytes: 0x04||X||Y

  let z = ecdh.computeSecret(recipientPubKey);
  if (z.length < 32) {
    z = Buffer.concat([Buffer.alloc(32 - z.length), z]); // left-pad, matches go's right-aligned copy into zeroed buffer
  } else if (z.length > 32) {
    z = z.subarray(z.length - 32);
  }

  // 2. concatKDF (NIST SP 800-56), kdLen=32, s1=nil -> single SHA-256(counter=1 || z) pass
  const counter = Buffer.from([0x00, 0x00, 0x00, 0x01]);
  const K = createHash('sha256').update(Buffer.concat([counter, z])).digest(); // 32 bytes
  const Ke = K.subarray(0, 16);
  const KmRaw = K.subarray(16, 32);
  const Km = createHash('sha256').update(KmRaw).digest(); // Km = SHA256(Km_raw), 32 bytes

  // 3. AES-128-CTR encrypt with Ke, random 16-byte IV
  const iv = randomBytes(16);
  const cipher = createCipheriv('aes-128-ctr', Ke, iv);
  const ciphertext = Buffer.concat([cipher.update(plaintext), cipher.final()]);
  const em = Buffer.concat([iv, ciphertext]);

  // 4. MAC = HMAC-SHA256(Km, em), s2=nil
  const mac = createHmac('sha256', Km).update(em).digest(); // 32 bytes

  return Buffer.concat([ephemPubKey, em, mac]);
}

function selfTest() {
  const pt = abiEncodeBoolUint256(true, 1n);
  if (pt.length !== 64) throw new Error(`self-test failed: plaintext length ${pt.length} != 64`);

  const ct = eciesEncrypt(pt, TEE_PUBKEY_X, TEE_PUBKEY_Y);
  const expectedLen = 65 + 16 + 64 + 32; // ephemPubKey + iv + ciphertext + mac
  if (ct.length !== expectedLen) {
    throw new Error(`self-test failed: ciphertext length ${ct.length} != ${expectedLen}`);
  }
  if (ct[0] !== 0x04) throw new Error('self-test failed: ephemPubKey does not start with 0x04');
}

function main() {
  const [, , isUpArg, amountArg] = process.argv;
  if (isUpArg === undefined || amountArg === undefined) {
    console.error('Usage: node encrypt_bet.mjs <true|false> <amount>');
    process.exit(1);
  }
  if (isUpArg !== 'true' && isUpArg !== 'false') {
    console.error('isUp must be exactly "true" or "false"');
    process.exit(1);
  }

  selfTest();

  const isUp = isUpArg === 'true';
  const amount = BigInt(amountArg);
  const plaintext = abiEncodeBoolUint256(isUp, amount);
  const ciphertext = eciesEncrypt(plaintext, TEE_PUBKEY_X, TEE_PUBKEY_Y);

  console.error(`[self-test OK] plaintext=64 bytes, ciphertext=${ciphertext.length} bytes (expected 177)`);
  console.log('0x' + ciphertext.toString('hex'));
}

main();
