# Welcome to Allod — A Guide for Ring Participants

If a friend or family member invited your Allod node into their **Ring**, welcome!

---

## What is an Allod Ring?

An Allod Ring is a private, reciprocal backup club between friends. Instead of paying monthly subscription fees to giant tech corporations, we share a small portion of our home storage with each other.

---

## Important Security Questions

### Can my friends see my photos or documents?
**No.** All data is encrypted on your home server *before* it leaves your house using industry-standard cryptography (AES-256 / ChaCha20). Your friends only store sealed, unreadable blocks of data.

### Can I see my friends' files?
**No.** Their backups on your drive are encrypted with their private passwords.

### What happens if my friend's computer gets a virus?
Allod stores peer backups in **append-only mode**. Even if a friend's node is attacked by ransomware, the attacker cannot delete or modify the historical backups stored on your machine.

### What should I do if my server goes offline?
If your server is disconnected or turned off for more than 48 hours, your friends' nodes will notice and display a friendly alert so you can check your Wi-Fi or power cable.
