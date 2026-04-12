# ภาพรวมของบริการ Incident Tracking Service

**GitHub Repository:** [https://github.com/krittamark/incident-tracking-service](https://github.com/krittamark/incident-tracking-service)

## Service Owner
นายกฤตเมธ ดำทองคำ (มาร์ค) รหัสนักศึกษา 6609490062 ภาคปกติ

## Service Purpose
IncidentTracking Service เป็นบริการที่ทำหน้าที่เป็นแหล่งข้อมูลหลักส่วนกลาง สำหรับการสร้างเหตุการณ์ฉุกเฉิน การจัดการสถานะวงจรชีวิตของเหตุการณ์ และการบันทึกลำดับเหตุการณ์ตามลำดับเวลา เพื่อประสานงานร่วมกับทุก Service ในระบบ

## Pain Point ที่แก้ไข
เวลาเกิดเหตุการณ์ภัยพิบัติ ข้อมูลมักกระจัดกระจายและขาดความน่าเชื่อถือ หน่วยงานที่เกี่ยวข้องขาดศูนย์กลางในการตรวจสอบว่าเกิดอะไรขึ้น ที่ไหน สถานะปัจจุบันคืออะไร และมีใครทำอะไรไปแล้วบ้าง ทำให้เกิดความซ้ำซ้อนและล่าช้า

## Target Users
Other Microservices บริการอื่น ๆ ในระบบ:
* **ReportVerify Service:** ส่งข้อมูลมาสร้าง Incident
* **MissionProgress Service:** ส่ง Log มาอัปเดต Timeline
* **Shelter Service / Donation Service:** ดึงสถานะไปตรวจสอบก่อนรับคน/รับบริจาค

## Service Boundary

**In-scope Responsibilities (สิ่งที่บริการนี้รับผิดชอบ)**
* จัดเก็บข้อมูลหลักของเหตุการณ์
* จัดการสถานะวงจรชีวิต
* จัดการประวัติการดำเนินการ

**Out-of-scope / Not Responsible For (ไม่รับผิดชอบ)**
* การคัดกรองข่าวปลอม (เป็นหน้าที่ของ ReportVerify Service)
* การประเมินความสำคัญของทีมกู้ภัย (เป็นหน้าที่ของ Rescue Prioritization Service)

## Autonomy / Decision Logic
* มีสิทธิ์ในการปฏิเสธการอัปเดตสถานะที่ข้ามขั้นตอน (เช่น จาก REPORTED กระโดดไป CLOSED โดยไม่มีการลงพื้นที่)
* ระบบจะทำการเพิ่ม Timeline Record อัตโนมัติในทุก ๆ ครั้งที่มีการเปลี่ยน Status

## Owned Data
* **Incident Master Data:** ข้อมูลหลักของเหตุการณ์ เช่น `incident_id`, `type`, `description`, `location`, `impact_level` ซึ่งบริการนี้เป็นผู้สร้าง และดูแลความถูกต้องแต่เพียงผู้เดียว
* **Incident Lifecycle State:** สถานะปัจจุบัน (`status`) ของภัยพิบัติที่เกิดขึ้น
* **Event Timeline Records:** อาร์เรย์ลำดับของเหตุการณ์ (`timeline`) ที่บันทึกประวัติการแก้ไขปัญหา

## Linked Data (Reference Only)
* `reported_by`: อาจจะอ้างอิงถึง user_id หรือชื่อ Service ที่แจ้งเข้า

## Non-Functional Requirements
* ระบบต้องพร้อมใช้งานตลอดเวลาเพื่อรับแจ้งเหตุ
* สถานะ และข้อมูลไทม์ไลน์ ต้องสอดคล้องกันเสมอ
* ต้องรองรับการส่งข้อมูลซ้ำโดยไม่ทำให้เกิด Incident หรือ Timeline ซ้ำซ้อน
* ต้องสามารถ Publish Event ไปยัง Message Broker ได้รวดเร็ว เพื่อให้ระบบแจ้งเตือนทำงานได้ทันที

---

## Service Data

### Incident Master Data (Owned by this service)

| Field Name | Type | Required | Description | Example |
| :--- | :--- | :---: | :--- | :--- |
| `incident_id` | UUIDv7 | Y | Primary Key | `019C774D-1AC5-75BB-AE95-5CD4AEB89258` |
| `incident_type` | string | Y | Category of incident | `flood`, `fire`, `generic` |
| `incident_description` | string | Y | รายละเอียดเหตุการณ์ | `Heavy rainfall caused flooding` |
| `exact_location` | string | Y | พิกัด Lat, Long | `13.7563, 100.5018` |
| `exact_location_description` | string | N | จุดสังเกตเพิ่มเติม | `ตรงข้ามเซเว่น ซอย5` |
| `impact_level` | integer | Y | ระดับผลกระทบ (1-4) | `3` |
| `priority` | string | Y | ความเร่งด่วน | `Low`, `Medium`, `High`, `Critical` |
| `status` | string | Y | สถานะปัจจุบัน | `REPORTED`, `DISPATCHED`, `ON-SITE`, `RESOLVED`, `CLOSED` |
| `reported_by` | string | Y | ต้นทางที่แจ้งเหตุ | `ReportVerify Service`, `Citizen_01` |
| `timeline` | JSON Array | Y | ลำดับเหตุการณ์ทั้งหมด | `[{"time": "...", "event": "...", "detail": "..."}]` |
| `created_at` | datetime | Y | เวลาที่สร้าง incident | `2026-02-22T00:00:00Z` |
| `updated_at` | datetime | Y | เวลาอัปเดตล่าสุด | `2026-02-22T00:15:00Z` |

---

## Service Architecture

Incident Tracking Service ถูกออกแบบมาภายใต้สถาปัตยกรรมแบบ Microservices โดยทำหน้าที่เป็นแหล่งข้อมูลหลักส่วนกลาง สำหรับการจัดการวงจรชีวิต และประวัติเหตุการณ์ฉุกเฉินทั้งหมดในระบบ

* **High Autonomy & Loose Coupling:** บริการนี้มีอำนาจตัดสินใจในข้อมูลของตนเองอย่างสมบูรณ์ และถูกออกแบบมาให้ไม่มีการเรียกไปยัง Service อื่นแบบ Synchronous เลย เพื่อให้ระบบสามารถรับโหลดและทำงานได้รวดเร็วที่สุดโดยไม่เกิดการพังตามระบบอื่น
* **Event-Driven Integration:** ใช้รูปแบบการทำงานแบบ Event-Driven สำหรับการสื่อสารขาออก โดยเมื่อมีการสร้างหรืออัปเดตเหตุการณ์ ระบบจะทำหน้าที่ Publish Event เข้าสู่ Message Broker เพื่อกระจายข้อมูลให้ Service อื่น ๆ ที่สนใจนำไปทำงานต่อทันที

---

## Service Interaction

การสื่อสารของ Incident Tracking Service แบ่งออกเป็น 2 รูปแบบหลัก คือ Inbound และ Outbound

### Synchronous Interaction (Inbound APIs - REST)
เปิดให้บริการ API เพื่อให้ Service อื่น ๆ เข้ามาจัดการหรือร้องขอข้อมูล
* **Create Incident (POST):** รับข้อมูลที่ผ่านการตรวจสอบและยืนยันแล้วจาก ReportVerify Service เพื่อสร้างเป็นเหตุการณ์ฉุกเฉินหลักในระบบ
* **Update Status & Timeline (PATCH):** รับข้อมูลการอัปเดตสถานะการปฏิบัติงานจากหน้างาน เพื่อเปลี่ยนสถานะและบันทึกลง Timeline
* **Get Incident Details (GET):** เปิดให้ Shelter Service และ Donation Service เข้ามาดึงข้อมูล และตรวจสอบสถานะของเหตุการณ์

### Asynchronous Interaction
ทำหน้าที่เป็น Producer ในการส่ง Message ออกไปยัง Channel/Queue `incident.core.events.v1` โดยไม่รอผลลัพธ์ตอบกลับ
* **Event INCIDENT_CREATED:** เมื่อสร้างเหตุการณ์สำเร็จ จะกระจาย Event ออกไปให้ Manage Dispatch Service, RecommendRescue Team Service และ EmergencyAlert Service / Notification เริ่มกระบวนการทำงานของตนเอง
* **Event STATUS_CHANGED:** เมื่อมีการอัปเดตสถานะ จะกระจาย Event ออกไปแบบ Real-time เพื่อให้ Donation Tracking Service และ Shelter Capacity Service รับทราบความคืบหน้าและนำไปตัดสินใจต่อ

---

## Dependency Mapping

### Upstream Dependencies
* **ReportVerify Service** พึ่งพา API ของเราในการบันทึกเหตุการณ์หลังจากคัดกรองข่าวปลอมสำเร็จ
* **MissionProgress Service** พึ่งพา API ของเราในการอัปเดตความคืบหน้าและสถานะการลงพื้นที่
* **Shelter Service & Donation Service** พึ่งพา API ของเราเพื่อตรวจสอบว่าภัยพิบัติอยู่ในสถานะใด

### Downstream Dependencies
* **Synchronous Dependencies:** ไม่มี เราไม่พึ่งพา API ของ Service อื่นเลยเพื่อลด Dependency และรักษาเสถียรภาพ
* **Infrastructure Dependencies:** พึ่งพาระบบ Message Broker (ตัวจัดการ Queue/Pub-Sub) สำหรับการส่ง Event `INCIDENT_CREATED` และ `STATUS_CHANGED` ออกไปยัง Service ปลายทาง
* **Asynchronous Consumers:** Manage Dispatch Service, Recommend Rescue Team Service, EmergencyAlert Service, Donation Tracking Service และ Shelter Capacity Service
