# Data Model

PublicIdentity {actor,name,public_key} excludes private key and filesystem locations.
OperationObservation {issue_id,event_id,actor,operation,intent,kind,timestamp,parents,state?,body?,request} comes from one verified StoredEvent; body is present for comments even when empty, state is present for rich issue revisions/openings. Snapshot belongs to read envelope, not original event.
DetailedIssue {id,creator,state,conflict,head_count} uses explicit null state under conflict. No new persisted type or hn/0 field.
