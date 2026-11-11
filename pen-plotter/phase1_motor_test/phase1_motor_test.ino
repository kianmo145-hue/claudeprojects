/*
 * Phase 1 - one DC gear motor as a position servo
 * Hardware: Arduino Uno R3 + L298N + homemade encoder (TCRT5000 / photo-interrupter)
 *
 * Strategy ("move, then correct"):
 *   1. run fast toward target, slow down in the creep zone
 *   2. brake, wait for the mechanics to settle
 *   3. if still off target, nudge with short pulses (max MAX_TRIES)
 *   4. final approach is always from the same direction -> gearbox
 *      backlash cancels out
 *
 * Encoder: single channel, mounted AFTER the gearbox. Direction is assumed
 * from the commanded direction (encDir). Count direction is therefore only
 * valid while we command the motor - good enough for phase 1.
 *
 * Serial commands (115200 baud, end with newline):
 *   M <counts>   absolute move            e.g.  M 400
 *   R <counts>   relative move            e.g.  R -120
 *   Z            zero the position counter
 *   P            print current position
 *   V <0..255>   set max PWM  (voltage cap - keep LOW for 6V TT motors!)
 *   W <0..255>   set creep PWM
 *   T <counts>   set tolerance
 *   C            calibration help: does R 1000, then measure travel in mm
 *                -> counts_per_mm = 1000 / measured_mm
 */

// ---------- pins ----------
const uint8_t PIN_ENA = 5;   // L298N ENA (PWM)
const uint8_t PIN_IN1 = 7;   // L298N IN1
const uint8_t PIN_IN2 = 8;   // L298N IN2
const uint8_t PIN_ENC = 2;   // encoder sensor digital out (interrupt pin)

// ---------- tuning ----------
int  maxPwm   = 140;   // voltage cap! ~9V supply -> ~5V effective at 140/255
int  creepPwm = 90;    // slow speed near target (raise if motor stalls)
long tol      = 1;     // acceptable error in counts
long slowZone = 15;    // start creeping this many counts before target
const int  SETTLE_MS        = 200;  // wait after stop before measuring
const int  NUDGE_MS         = 30;   // length of one correction pulse
const int  MAX_TRIES        = 10;   // correction attempts per move
const long BACKLASH_BACKOFF = 30;   // counts; final approach always from below
const unsigned long DEBOUNCE_US = 300; // min time between encoder edges

// ---------- state ----------
volatile long          pos        = 0;
volatile int8_t        encDir     = 1;
volatile unsigned long lastEdgeUs = 0;

void encIsr() {
  unsigned long now = micros();
  if (now - lastEdgeUs < DEBOUNCE_US) return;  // noise filter
  lastEdgeUs = now;
  pos += encDir;
}

long readPos() {
  noInterrupts();
  long p = pos;
  interrupts();
  return p;
}

// pwm > 0 forward, pwm < 0 reverse, 0 = coast
void driveRaw(int pwm) {
  if (pwm > 0) {
    encDir = 1;
    digitalWrite(PIN_IN1, HIGH); digitalWrite(PIN_IN2, LOW);
    analogWrite(PIN_ENA, min(pwm, maxPwm));
  } else if (pwm < 0) {
    encDir = -1;
    digitalWrite(PIN_IN1, LOW); digitalWrite(PIN_IN2, HIGH);
    analogWrite(PIN_ENA, min(-pwm, maxPwm));
  } else {
    digitalWrite(PIN_IN1, LOW); digitalWrite(PIN_IN2, LOW);
    analogWrite(PIN_ENA, 0);
  }
}

// active brake on L298N: IN1 = IN2 = HIGH
void stopBrake() {
  digitalWrite(PIN_IN1, HIGH); digitalWrite(PIN_IN2, HIGH);
  analogWrite(PIN_ENA, 255);
  delay(40);
  driveRaw(0);
}

// run toward target, creep near it, brake when reached/passed
void moveRough(long target) {
  long err = target - readPos();
  if (err == 0) return;
  int dir = (err > 0) ? 1 : -1;
  unsigned long tStart = millis();
  while (true) {
    err = target - readPos();
    if ((dir > 0 && err <= 0) || (dir < 0 && err >= 0)) break;
    driveRaw(dir * ((labs(err) > slowZone) ? maxPwm : creepPwm));
    if (millis() - tStart > 15000) {            // stall safety
      Serial.println(F("!! timeout - check motor/encoder"));
      break;
    }
  }
  stopBrake();
}

void nudge(long err) {
  int dir = (err > 0) ? 1 : -1;
  driveRaw(dir * creepPwm);
  delay(NUDGE_MS);
  stopBrake();
}

// full move with backlash compensation + correction loop
void goTo(long target) {
  if (target < readPos()) {
    moveRough(target - BACKLASH_BACKOFF);   // overshoot low, approach upward
  }
  moveRough(target);
  for (int i = 0; i < MAX_TRIES; i++) {
    delay(SETTLE_MS);
    long err = target - readPos();
    if (labs(err) <= tol) break;
    nudge(err);
  }
  long err = target - readPos();
  Serial.print(F("pos="));  Serial.print(readPos());
  Serial.print(F(" err=")); Serial.println(err);
}

void setup() {
  pinMode(PIN_ENA, OUTPUT);
  pinMode(PIN_IN1, OUTPUT);
  pinMode(PIN_IN2, OUTPUT);
  pinMode(PIN_ENC, INPUT_PULLUP);
  driveRaw(0);
  attachInterrupt(digitalPinToInterrupt(PIN_ENC), encIsr, CHANGE);
  Serial.begin(115200);
  Serial.println(F("phase1 ready. commands: M R Z P V W T C"));
}

void loop() {
  if (!Serial.available()) return;
  String line = Serial.readStringUntil('\n');
  line.trim();
  if (line.length() == 0) return;
  char cmd = toupper(line.charAt(0));
  long val = line.substring(1).toInt();

  switch (cmd) {
    case 'M': goTo(val); break;
    case 'R': goTo(readPos() + val); break;
    case 'Z': noInterrupts(); pos = 0; interrupts();
              Serial.println(F("zeroed")); break;
    case 'P': Serial.print(F("pos=")); Serial.println(readPos()); break;
    case 'V': maxPwm = constrain(val, 0, 255);
              Serial.print(F("maxPwm=")); Serial.println(maxPwm); break;
    case 'W': creepPwm = constrain(val, 0, 255);
              Serial.print(F("creepPwm=")); Serial.println(creepPwm); break;
    case 'T': tol = max(0L, val);
              Serial.print(F("tol=")); Serial.println(tol); break;
    case 'C': Serial.println(F("calibration: moving +1000 counts."));
              Serial.println(F("measure travel in mm -> counts_per_mm = 1000/mm"));
              goTo(readPos() + 1000); break;
    default:  Serial.println(F("? unknown command")); break;
  }
}
