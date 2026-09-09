# Research Decisions

- Reuse captured Identity through append: pin controls the signer actually used and cannot be satisfied by an earlier unrelated identity read. Alternative global-identity recheck would not bind append and is rejected.
- Query original verified events rather than mutable catalog state: exact replay includes parents/content/time; current state loses that information. No storage added.
- Add generic details mode: summary-only list forces consumers into N verified scans. Full-state pages reuse one bounded reader pass and retain original default behavior.
- No dependencies introduced. Supply-chain challenge is not applicable; independent implementation review remains required for the changed trust boundary.
