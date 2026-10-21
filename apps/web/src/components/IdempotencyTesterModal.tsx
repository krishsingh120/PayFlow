import React, { useState } from 'react';
import { X, Play } from 'lucide-react';
import { createPayment } from '../lib/api';
import type { Payment, ApiResponse } from '../lib/api';

interface IdempotencyTesterModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

interface StepResult {
  step: number;
  title: string;
  expectedStatus: number;
  actualStatus?: number;
  statusText: string;
  response?: ApiResponse<Payment>;
  passed?: boolean;
}

export const IdempotencyTesterModal: React.FC<IdempotencyTesterModalProps> = ({
  isOpen,
  onClose,
  onSuccess,
}) => {
  const [running, setRunning] = useState(false);
  const [testKey, setTestKey] = useState(() => `idem_lab_${Date.now().toString(36)}`);
  const [results, setResults] = useState<StepResult[] | null>(null);

  if (!isOpen) return null;

  const runSimulation = async () => {
    setRunning(true);
    const key = `idem_lab_${Date.now().toString(36)}`;
    setTestKey(key);

    const stepResults: StepResult[] = [
      {
        step: 1,
        title: 'Initial Payment Request',
        expectedStatus: 201,
        statusText: '201 Created (New Record)',
      },
      {
        step: 2,
        title: 'Network Retry Replay (Identical Payload)',
        expectedStatus: 200,
        statusText: '200 OK (Idempotent Replay)',
      },
      {
        step: 3,
        title: 'Payload Tampering (Altered Amount with Same Key)',
        expectedStatus: 409,
        statusText: '409 Conflict (Idempotency Mismatch)',
      },
    ];

    // ── Step 1: Initial Payment ─────────────────────────────────────────────
    const res1 = await createPayment(key, {
      amount: 25000, // ₹250.00
      currency: 'INR',
      description: 'Idempotency Lab Test #1',
      metadata: { test_step: 1 },
    });
    stepResults[0].actualStatus = res1.status;
    stepResults[0].response = res1;
    stepResults[0].passed = res1.status === 201;
    setResults([...stepResults]);

    // Short pause for realistic demonstration
    await new Promise((r) => setTimeout(r, 600));

    // ── Step 2: Idempotent Replay ───────────────────────────────────────────
    const res2 = await createPayment(key, {
      amount: 25000, // Identical!
      currency: 'INR',
      description: 'Idempotency Lab Test #1',
      metadata: { test_step: 1 },
    });
    stepResults[1].actualStatus = res2.status;
    stepResults[1].response = res2;
    stepResults[1].passed = res2.status === 200 && res2.data?.id === res1.data?.id;
    setResults([...stepResults]);

    await new Promise((r) => setTimeout(r, 600));

    // ── Step 3: Conflict Rejection ──────────────────────────────────────────
    const res3 = await createPayment(key, {
      amount: 99900, // Different amount (₹999.00)!
      currency: 'INR',
      description: 'Tampered Payload',
    });
    stepResults[2].actualStatus = res3.status;
    stepResults[2].response = res3;
    stepResults[2].passed = res3.status === 409;
    setResults([...stepResults]);

    setRunning(false);
    onSuccess();
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content" style={{ maxWidth: 680 }} onClick={(e) => e.stopPropagation()}>
        {/* Header */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
          <div>
            <h3 style={{ fontSize: 18, fontWeight: 600, color: '#F8FAFC' }}>
              Idempotency Verification Lab
            </h3>
            <p style={{ fontSize: 13, color: 'var(--text-dim)' }}>
              Interactive validation of Pine Labs payment gateway guarantees
            </p>
          </div>
          <button
            onClick={onClose}
            style={{ background: 'none', border: 'none', color: 'var(--text-dim)', cursor: 'pointer' }}
          >
            <X size={20} />
          </button>
        </div>

        {/* Explanation banner */}
        <div
          style={{
            background: 'rgba(16, 185, 129, 0.08)',
            border: '1px solid rgba(16, 185, 129, 0.25)',
            borderRadius: 'var(--radius-md)',
            padding: 14,
            marginBottom: 20,
            fontSize: 13,
            color: 'var(--text-muted)',
            lineHeight: 1.5,
          }}
        >
          <strong style={{ color: 'var(--emerald-400)' }}>Interview Talking Point:</strong> PayFlow enforces strict
          financial idempotency via unique constraints and state verification. In this lab, we test initial creation,
          network replay safety (returning cached payment without double charge), and conflict rejection if payload parameters
          differ.
        </div>

        {/* Run trigger */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 20 }}>
          <div style={{ fontFamily: 'var(--font-mono)', fontSize: 12, color: 'var(--text-dim)' }}>
            Active Key: <span style={{ color: '#F8FAFC' }}>{testKey}</span>
          </div>
          <button
            className="btn btn-primary"
            onClick={runSimulation}
            disabled={running}
            style={{ padding: '8px 16px' }}
          >
            <Play size={14} />
            {running ? 'Executing Tests...' : 'Execute Live Scenario'}
          </button>
        </div>

        {/* Results Timeline */}
        {results ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            {results.map((r) => (
              <div
                key={r.step}
                style={{
                  background: 'rgba(15, 23, 42, 0.7)',
                  border: `1px solid ${
                    r.passed === undefined
                      ? 'var(--border-subtle)'
                      : r.passed
                      ? 'rgba(16, 185, 129, 0.4)'
                      : 'rgba(244, 63, 94, 0.4)'
                  }`,
                  borderRadius: 'var(--radius-md)',
                  padding: 16,
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 6 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                    <div
                      style={{
                        width: 24,
                        height: 24,
                        borderRadius: '50%',
                        background:
                          r.passed === undefined
                            ? 'rgba(255, 255, 255, 0.1)'
                            : r.passed
                            ? 'var(--emerald-500)'
                            : '#F43F5E',
                        color: '#FFFFFF',
                        fontSize: 12,
                        fontWeight: 700,
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                      }}
                    >
                      {r.step}
                    </div>
                    <span style={{ fontWeight: 600, fontSize: 14, color: '#F8FAFC' }}>{r.title}</span>
                  </div>

                  <span
                    style={{
                      fontSize: 12,
                      fontWeight: 600,
                      fontFamily: 'var(--font-mono)',
                      color:
                        r.actualStatus === r.expectedStatus
                          ? 'var(--emerald-400)'
                          : r.actualStatus === undefined
                          ? 'var(--text-dim)'
                          : '#F43F5E',
                    }}
                  >
                    {r.actualStatus ? `HTTP ${r.actualStatus}` : 'Pending...'}
                  </span>
                </div>

                <div style={{ fontSize: 12, color: 'var(--text-dim)', marginLeft: 34 }}>
                  Expected: <span style={{ color: 'var(--text-muted)' }}>{r.statusText}</span>
                  {r.response && (
                    <span style={{ marginLeft: 12 }}>
                      Latency: <strong style={{ color: '#F8FAFC' }}>{r.response.latencyMs}ms</strong>
                    </span>
                  )}
                </div>

                {r.response?.message && (
                  <div
                    style={{
                      marginTop: 8,
                      marginLeft: 34,
                      fontSize: 12,
                      fontFamily: 'var(--font-mono)',
                      color: r.passed ? 'var(--emerald-400)' : '#F43F5E',
                    }}
                  >
                    Response: {r.response.message}
                  </div>
                )}
              </div>
            ))}
          </div>
        ) : (
          <div
            style={{
              padding: '36px 16px',
              textAlign: 'center',
              color: 'var(--text-dim)',
              border: '1px dashed var(--border-subtle)',
              borderRadius: 'var(--radius-md)',
            }}
          >
            Click "Execute Live Scenario" to trigger 3 sequential HTTP calls to the payment gateway engine.
          </div>
        )}

        <div style={{ marginTop: 24, display: 'flex', justifyContent: 'flex-end' }}>
          <button className="btn btn-secondary" onClick={onClose}>
            Close Lab
          </button>
        </div>
      </div>
    </div>
  );
};
