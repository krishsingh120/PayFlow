import { useState, useEffect, useCallback } from 'react';
import { Header } from './components/Header';
import { MetricsGrid } from './components/MetricsGrid';
import { TransactionList } from './components/TransactionList';
import { CreatePaymentModal } from './components/CreatePaymentModal';
import { IdempotencyTesterModal } from './components/IdempotencyTesterModal';
import { PaymentDetailsModal } from './components/PaymentDetailsModal';
import { listPayments } from './lib/api';
import type { Payment } from './lib/api';

export function App() {
  const [payments, setPayments] = useState<Payment[]>([]);
  const [total, setTotal] = useState(0);
  const [selectedStatus, setSelectedStatus] = useState('ALL');
  const [searchQuery, setSearchQuery] = useState('');
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);

  // Modals state
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [isIdempOpen, setIsIdempOpen] = useState(false);
  const [inspectPayment, setInspectPayment] = useState<Payment | null>(null);

  const fetchData = useCallback(async () => {
    setIsRefreshing(true);
    const res = await listPayments({
      limit: 100,
      status: selectedStatus === 'ALL' ? undefined : selectedStatus,
    });
    setIsRefreshing(false);

    if (res.status === 200 && res.data) {
      setPayments(res.data.data);
      setTotal(res.data.total);
    }
  }, [selectedStatus]);

  // Initial fetch and dependency on status filter
  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Auto-polling effect (every 3 seconds if enabled)
  useEffect(() => {
    if (!autoRefresh) return;
    const interval = setInterval(() => {
      fetchData();
    }, 3000);
    return () => clearInterval(interval);
  }, [autoRefresh, fetchData]);

  return (
    <div style={{ minHeight: '100vh', display: 'flex', flexDirection: 'column' }}>
      <Header
        onOpenCreateModal={() => setIsCreateOpen(true)}
        onOpenIdempotencyLab={() => setIsIdempOpen(true)}
        onRefreshData={fetchData}
        isRefreshing={isRefreshing}
      />

      <main style={{ maxWidth: 1280, margin: '0 auto', width: '100%', padding: '24px', flex: 1 }}>
        {/* Banner with Pine Labs Architecture Context */}
        <div
          style={{
            background: 'linear-gradient(90deg, rgba(16, 185, 129, 0.12) 0%, rgba(56, 189, 248, 0.08) 100%)',
            border: '1px solid rgba(16, 185, 129, 0.25)',
            borderRadius: 'var(--radius-lg)',
            padding: '16px 20px',
            marginBottom: 24,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 16,
            flexWrap: 'wrap',
          }}
        >
          <div>
            <h1 style={{ fontSize: 16, fontWeight: 600, color: '#F8FAFC', marginBottom: 2 }}>
              PayFlow Gateway & Core Transaction Engine
            </h1>
            <p style={{ fontSize: 13, color: 'var(--text-muted)' }}>
              Dual-layer architecture: Express Gateway (proxy, CORS, 200/min rate limit) → Go Payment Core (ULID PKs, paise math, state machine).
            </p>
          </div>
          <div style={{ display: 'flex', gap: 10 }}>
            <span style={{ fontSize: 12, fontFamily: 'var(--font-mono)', color: 'var(--emerald-400)', background: 'rgba(16, 185, 129, 0.1)', padding: '4px 10px', borderRadius: 6, border: '1px solid rgba(16, 185, 129, 0.3)' }}>
              API: :3000/api
            </span>
            <span style={{ fontSize: 12, fontFamily: 'var(--font-mono)', color: '#38BDF8', background: 'rgba(56, 189, 248, 0.1)', padding: '4px 10px', borderRadius: 6, border: '1px solid rgba(56, 189, 248, 0.3)' }}>
              Go Core: :8080
            </span>
          </div>
        </div>

        {/* Financial Metrics Grid */}
        <MetricsGrid payments={payments} total={total} />

        {/* Live Transaction Ledger */}
        <TransactionList
          payments={payments}
          selectedStatus={selectedStatus}
          onSelectStatus={setSelectedStatus}
          onInspect={setInspectPayment}
          autoRefresh={autoRefresh}
          onToggleAutoRefresh={() => setAutoRefresh((prev) => !prev)}
          searchQuery={searchQuery}
          onSearchChange={setSearchQuery}
        />
      </main>

      {/* Modals */}
      <CreatePaymentModal
        isOpen={isCreateOpen}
        onClose={() => setIsCreateOpen(false)}
        onSuccess={fetchData}
      />

      <IdempotencyTesterModal
        isOpen={isIdempOpen}
        onClose={() => setIsIdempOpen(false)}
        onSuccess={fetchData}
      />

      <PaymentDetailsModal
        payment={inspectPayment}
        onClose={() => setInspectPayment(null)}
      />
    </div>
  );
}

export default App;
