/*
The package sds implements everything that is necessary for sending and receiving SDS messages through the
Peripheral Equipment Interface (PEI) of a TETRA radio terminal. This implementation is solely based on:
  [AI]  ETSI TS 100 392-2 V3.9.2 (2020-06)
  [PEI] ETSI EN 300 392-5 V2.7.1 (2020-04)

The most relevant chapters in [AI] are 29 (SDS-TL Protocol) and 14 (CMCE Protocol).

Abbreviations:
PDU: Protocol Data Unit
SDU: Service Data Unit
UDH: User Data Header

Restrictions:
Store/forward control information is not supported yet.

*/
package sds
