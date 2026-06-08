# Licensing — GNU AGPL v3

GiraffeMail Archive is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**.

Full legal text: [LICENSE](../LICENSE) · [GNU AGPL v3](https://www.gnu.org/licenses/agpl-3.0.html)

---

## What “free software” means here

AGPL-3.0 is a **copyleft** license. You may use, study, modify, and redistribute the software — including charging money for copies or support — but you must preserve users’ freedoms and share source code under the same license when the rules below apply.

“Free” refers to **freedom**, not price.

---

## Quick answers

| Question | Answer |
|----------|--------|
| Can I use it internally at my company? | **Yes.** No fee required. |
| Can I modify it? | **Yes.** You must document changes and comply with AGPL when you distribute or offer network access (see below). |
| Can I sell copies or support? | **Yes.** You may charge for binaries, hosting, integration, or consulting. |
| Can I resell it as a product? | **Yes**, if you comply with AGPL — especially **source code availability** for users of the modified version. |
| Can I run it as a SaaS for customers? | **Yes**, but AGPL **§13** requires you to offer corresponding source code to users interacting with the service over a network. |
| Can I make a proprietary fork without sharing source? | **No** — not if you distribute it or run it as a network service for others without providing source under AGPL. |
| Can I link proprietary code into it? | **Careful.** AGPL extends to combined works. Proprietary modules that form a single program with GiraffeMail may need to be AGPL-licensed too. Consult a lawyer for combined/derivative works. |
| Can I use the name “GiraffeMail” for my fork? | Trademarks are separate from copyright. Do not imply official Giraffe.ge endorsement unless permitted. |

---

## When you must share source (AGPL highlights)

You must provide **Corresponding Source** under AGPL-3.0 when you:

1. **Distribute** copies of the software (binary or source), or  
2. **Offer network interaction** — users access a modified version over a network (e.g. SaaS, managed hosting, multi-tenant appliance with remote UI).

### Minimum obligations

- Include the [LICENSE](../LICENSE) and copyright notice.
- Provide complete corresponding source (build scripts, frontend, migrations, etc.).
- License your modifications under AGPL-3.0.
- For network use (§13): offer source to users who interact with the program — typically via a download link in the UI or documentation.

### What you do **not** need to share

- Your **separate** programs that merely **aggregate** with GiraffeMail (e.g. call its HTTP API from another app) are generally not required to become AGPL — unless they form a single combined program. API-only integration is usually fine; embedding or merging codebases is not.

---

## Modify and redistribute — checklist

If you ship a modified GiraffeMail (on-prem binary, Docker image, appliance, or hosted service):

- [ ] Keep `LICENSE` and state you modified the program  
- [ ] Publish corresponding source (Git repo, tarball, or written offer valid ≥3 years)  
- [ ] Include instructions to build (`Makefile`, `go.mod`, `frontend/`)  
- [ ] For SaaS: provide a clear **“Source code”** link for network users  
- [ ] Do not remove copyright notices from source files you retain  

---

## Commercial licensing

If AGPL terms do not fit your use case (e.g. OEM embedding without source disclosure), contact **Giraffe.ge** for a separate commercial license. AGPL is the default for this repository.

---

## Third-party components

GiraffeMail depends on open-source libraries (Go modules, Next.js, etc.), each under its own license. See `go.mod` and `frontend/package.json`. Your distribution must comply with those licenses as well.

---

## Disclaimer

This document is a **summary** for convenience, not legal advice. The [LICENSE](../LICENSE) file and [GNU AGPL v3](https://www.gnu.org/licenses/agpl-3.0.html) are authoritative.
