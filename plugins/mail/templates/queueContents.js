const mailQueue = [
    {"queue_name": "active", "queue_id": "A1B2C3D4E5F6", "arrival_time": 1693203845, "message_size": 20345, "forced_expire": false, "sender": "alice@example.com", "recipients": [{"address": "bob@mail.com", "delay_reason": "host alt1.gmail-smtp-in.l.google.com[142.250.153.26] said: 452-4.2.2 The recipient's inbox is out of storage space."}]},
    {"queue_name": "deferred", "queue_id": "F6E5D4C3B2A1", "arrival_time": 1694203845, "message_size": 15367, "forced_expire": true, "sender": "carol@example.com", "recipients": [{"address": "dave@mail.com", "delay_reason": "host alt2.gmail-smtp-in.l.google.com[142.250.153.27] said: 451 4.3.0 Temporary system problem. Try again later."}]},
    {"queue_name": "hold", "queue_id": "1A2B3C4D5E6F", "arrival_time": 1692203845, "message_size": 25678, "forced_expire": false, "sender": "eve@example.com", "recipients": [{"address": "alice@test.org", "delay_reason": "host alt3.gmail-smtp-in.l.google.com[142.250.153.28] said: 450 4.2.1 Mailbox unavailable."}]},
    {"queue_name": "active", "queue_id": "6F5E4D3C2B1A", "arrival_time": 1691203845, "message_size": 17892, "forced_expire": true, "sender": "bob@mail.com", "recipients": [{"address": "carol@test.org", "delay_reason": "host alt4.gmail-smtp-in.l.google.com[142.250.153.29] said: 554 5.7.1 Message blocked due to content restrictions."}]},
    {"queue_name": "deferred", "queue_id": "B2A1C3D4E5F6", "arrival_time": 1690103845, "message_size": 14095, "forced_expire": false, "sender": "dave@mail.com", "recipients": [{"address": "eve@test.org", "delay_reason": "host alt1.gmail-smtp-in.l.google.com[142.250.153.26] said: 452-4.2.2 The recipient's inbox is out of storage space."}]},
    {"queue_name": "hold", "queue_id": "2B1A3C4D5E6F", "arrival_time": 1689203845, "message_size": 26789, "forced_expire": true, "sender": "alice@mail.com", "recipients": [{"address": "bob@test.org", "delay_reason": "host alt2.gmail-smtp-in.l.google.com[142.250.153.27] said: 451 4.3.0 Temporary system problem. Try again later."}]},
    {"queue_name": "active", "queue_id": "C3B2A1D4E5F6", "arrival_time": 1688203845, "message_size": 19876, "forced_expire": false, "sender": "carol@mail.com", "recipients": [{"address": "dave@test.org", "delay_reason": "host alt3.gmail-smtp-in.l.google.com[142.250.153.28] said: 450 4.2.1 Mailbox unavailable."}]},
    {"queue_name": "deferred", "queue_id": "3C2B1A4D5E6F", "arrival_time": 1687203845, "message_size": 15234, "forced_expire": true, "sender": "eve@mail.com", "recipients": [{"address": "alice@test.net", "delay_reason": "host alt4.gmail-smtp-in.l.google.com[142.250.153.29] said: 554 5.7.1 Message blocked due to content restrictions."}]},
    {"queue_name": "hold", "queue_id": "D4C3B2A1E5F6", "arrival_time": 1686203845, "message_size": 16789, "forced_expire": false, "sender": "bob@test.net", "recipients": [{"address": "carol@test.net", "delay_reason": "host alt1.gmail-smtp-in.l.google.com[142.250.153.26] said: 452-4.2.2 The recipient's inbox is out of storage space."}]},
    {"queue_name": "active", "queue_id": "4D3C2B1A5E6F", "arrival_time": 1685203845, "message_size": 23956, "forced_expire": true, "sender": "dave@test.net", "recipients": [{"address": "eve@test.net", "delay_reason": "host alt2.gmail-smtp-in.l.google.com[142.250.153.27] said: 451 4.3.0 Temporary system problem. Try again later."}]}
];

const fields = {
    "recipients": {"label": "Recipients", "visible": true},
    "sender": {"label": "Sender", "visible": true},
    "arrival_time": {"label": "When", "visible": true},
    "queue_id": {"label": "ID", "visible": true},
    "message_size": {"label": "Message Size", "visible": false},
    "delay_reason": {"label": "Delay Reason", "visible": false},
    "forced_expire": {"label": "Forced Expire", "visible": false},
    "queue_name": {"label": "Queue", "visible": false}
};

const timeFormat = "d-m"; // Custom time format

