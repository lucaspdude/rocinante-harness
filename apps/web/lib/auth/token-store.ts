// Token store backed by IndexedDB. Tokens are encrypted at rest using
// a key derived from a "wcphrase" (set once at first login and
// stored in localStorage as rh-wcphrase-v1).
import { openDB } from "idb";

const DB_NAME = "rh-auth";
const DB_VERSION = 1;
const STORE = "tokens";

export interface StoredTokens {
  access_token: string;
  refresh_token: string;
  device_id: string;
}

const WC_PHRASE_KEY = "rh-wcphrase-v1";

function getWcPhrase(): string {
  if (typeof window === "undefined") return "";
  let phrase = window.localStorage.getItem(WC_PHRASE_KEY);
  if (!phrase) {
    const buf = new Uint8Array(32);
    crypto.getRandomValues(buf);
    phrase = Array.from(buf, (b) => b.toString(16).padStart(2, "0")).join("");
    window.localStorage.setItem(WC_PHRASE_KEY, phrase);
  }
  return phrase;
}

async function deriveKey(phrase: string, salt: Uint8Array): Promise<CryptoKey> {
  const enc = new TextEncoder();
  const baseKey = await crypto.subtle.importKey(
    "raw",
    enc.encode(phrase),
    "PBKDF2",
    false,
    ["deriveKey"]
  );
  return crypto.subtle.deriveKey(
    {
      name: "PBKDF2",
      salt: salt as BufferSource,
      iterations: 100_000,
      hash: "SHA-256",
    },
    baseKey,
    { name: "AES-GCM", length: 256 },
    false,
    ["encrypt", "decrypt"]
  );
}

async function encrypt(value: string): Promise<{ ciphertext: ArrayBuffer; iv: Uint8Array; salt: Uint8Array }> {
  const phrase = getWcPhrase();
  const salt = crypto.getRandomValues(new Uint8Array(16));
  const iv = crypto.getRandomValues(new Uint8Array(12));
  const key = await deriveKey(phrase + ":" + Array.from(salt).join(","), salt);
  const ciphertext = await crypto.subtle.encrypt(
    { name: "AES-GCM", iv: iv as BufferSource },
    key,
    new TextEncoder().encode(value)
  );
  return { ciphertext, iv, salt };
}

async function decrypt(payload: { ciphertext: ArrayBuffer; iv: Uint8Array; salt: Uint8Array }): Promise<string> {
  const phrase = getWcPhrase();
  const key = await deriveKey(phrase + ":" + Array.from(payload.salt).join(","), payload.salt);
  const plain = await crypto.subtle.decrypt(
    { name: "AES-GCM", iv: payload.iv as BufferSource },
    key,
    payload.ciphertext
  );
  return new TextDecoder().decode(plain);
}

interface StoredRow {
  ciphertext: ArrayBuffer;
  iv: Uint8Array;
  salt: Uint8Array;
}

export class TokenStore {
  async save(tokens: StoredTokens): Promise<void> {
    const db = await openDB(DB_NAME, DB_VERSION, {
      upgrade(db) {
        if (!db.objectStoreNames.contains(STORE)) {
          db.createObjectStore(STORE);
        }
      },
    });
    const payload = JSON.stringify(tokens);
    const enc = await encrypt(payload);
    const row: StoredRow = {
      ciphertext: enc.ciphertext,
      iv: enc.iv,
      salt: enc.salt,
    };
    await db.put(STORE, row, "current");
  }

  async load(): Promise<StoredTokens | null> {
    const db = await openDB(DB_NAME, DB_VERSION);
    const row = (await db.get(STORE, "current")) as StoredRow | undefined;
    if (!row) return null;
    try {
      const json = await decrypt(row);
      return JSON.parse(json) as StoredTokens;
    } catch {
      return null;
    }
  }

  async clear(): Promise<void> {
    const db = await openDB(DB_NAME, DB_VERSION);
    await db.delete(STORE, "current");
  }
}

export const tokenStore = new TokenStore();
