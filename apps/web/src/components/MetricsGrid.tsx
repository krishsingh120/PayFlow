import React from 'react';
import { IndianRupee, Layers, CheckCircle2, Clock } from 'lucide-react';
import { formatINR } from '../lib/api';
import type { Payment } from '../lib/api';

interface MetricsGridProps {
  payments: Payment[];
  total: number;
}

export const MetricsGrid: React.FC<MetricsGridProps> = ({ payments, total }) => {
  const totalVolumePaise = payments
    .filter((p) => p.status === 'SUCCEEDED' || p.status === 'CREATED' || p.status === 'PENDING')
    .reduce((sum, p) => sum + p.amount, 0);

  const succeededCount = payments.filter((p) => p.status === 'SUCCEEDED').length;
  const pendingCount = payments.filter((p) => p.status === 'PENDING' || p.status === 'CREATED').length;
  const successRate = total > 0 ? Math.round((succeededCount / total) * 100) : 100;

  const metrics = [
    {
      label: 'Total Processed Volume',
      value: formatINR(totalVolumePaise),
      subtext: `${payments.length} loaded records`,
      icon: IndianRupee,
      color: 'var(--emerald-400)',
      bg: 'rgba(16, 185, 129, 0.12)',
    },
    {
      label: 'Total Transactions',
      value: total.toLocaleString(),
      subtext: 'Registered idempotency keys',
      icon: Layers,
      color: '#38BDF8',
      bg: 'rgba(56, 189, 248, 0.12)',
    },
    {
      label: 'Success Rate',
      value: `${successRate}%`,
      subtext: `${succeededCount} succeeded payments`,
      icon: CheckCircle2,
      color: '#10B981',
      bg: 'rgba(16, 185, 129, 0.12)',
    },
    {
      label: 'In-Flight / Pending',
      value: pendingCount.toString(),
      subtext: 'Awaiting provider resolution',
      icon: Clock,
      color: '#F59E0B',
      bg: 'rgba(245, 158, 11, 0.12)',
    },
  ];

  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
        gap: 16,
        marginBottom: 24,
      }}
    >
      {metrics.map((m, idx) => {
        const Icon = m.icon;
        return (
          <div key={idx} className="glass-panel" style={{ padding: '20px 22px' }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
              <span style={{ fontSize: 13, color: 'var(--text-muted)', fontWeight: 500 }}>
                {m.label}
              </span>
              <div
                style={{
                  width: 34,
                  height: 34,
                  borderRadius: 8,
                  background: m.bg,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                }}
              >
                <Icon size={18} color={m.color} />
              </div>
            </div>
            <div style={{ fontSize: 26, fontWeight: 700, color: '#F8FAFC', letterSpacing: '-0.02em', marginBottom: 4 }}>
              {m.value}
            </div>
            <div style={{ fontSize: 12, color: 'var(--text-dim)' }}>{m.subtext}</div>
          </div>
        );
      })}
    </div>
  );
};
