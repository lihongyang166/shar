``
message ProcessHistoryEntry {
    bool compensating = 12;
``

``
message Element {
 X Targets compensation = 22; // Compensation transitions
  bool isForCompensation = 23; // IsForCompensation - this element is used for compensation activities.
``
