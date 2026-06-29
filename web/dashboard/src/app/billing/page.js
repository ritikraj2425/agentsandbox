'use client';

import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { CreditCard, Check, Zap, Building2, Loader2, ShieldCheck } from 'lucide-react';

export default function BillingPage() {
  const [currentPlan, setCurrentPlan] = useState('developer');
  const [isProcessing, setIsProcessing] = useState(false);
  const [showRazorpayMock, setShowRazorpayMock] = useState(false);
  const [selectedPlanToBuy, setSelectedPlanToBuy] = useState(null);

  const plans = [
    {
      id: 'developer',
      name: 'Developer',
      price: '$0',
      period: 'forever',
      description: 'Perfect for prototyping and local evaluation.',
      features: [
        '10 concurrent sessions',
        'Docker & local runtimes',
        'Community support',
        'Basic metrics'
      ],
      icon: Zap,
    },
    {
      id: 'pro',
      name: 'Pro',
      price: '$99',
      period: 'per month',
      description: 'For startups scaling their AI agents in production.',
      features: [
        '100 concurrent sessions',
        'Firecracker MicroVMs',
        'gVisor Sandbox',
        'Priority email support',
        'Advanced Network Policies'
      ],
      icon: ShieldCheck,
    },
    {
      id: 'enterprise',
      name: 'Enterprise',
      price: 'Custom',
      period: '',
      description: 'Dedicated clusters and SLA guarantees.',
      features: [
        'Unlimited concurrent sessions',
        'VPC Peering',
        'Single Sign-On (SSO)',
        'Dedicated Slack channel',
        'Custom Runtime Images'
      ],
      icon: Building2,
    }
  ];

  function handleSubscribeClick(planId) {
    if (planId === currentPlan) return;
    if (planId === 'enterprise') {
      window.location.href = 'mailto:sales@agentsandbox.com';
      return;
    }
    
    setSelectedPlanToBuy(plans.find(p => p.id === planId));
    setShowRazorpayMock(true);
  }

  function simulateRazorpaySuccess() {
    setIsProcessing(true);
    setTimeout(() => {
      setIsProcessing(false);
      setShowRazorpayMock(false);
      setCurrentPlan(selectedPlanToBuy.id);
      setSelectedPlanToBuy(null);
    }, 2000);
  }

  return (
    <div className="p-8 max-w-6xl mx-auto">
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-foreground flex items-center gap-3">
          <CreditCard className="text-primary" size={28} />
          Billing & Subscriptions
        </h1>
        <p className="text-muted mt-2 text-lg">Manage your plan, limits, and payment methods.</p>
      </div>

      <div className="bg-surface border border-border rounded-xl p-6 mb-12 shadow-sm flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold text-foreground mb-1">Current Plan: <span className="text-primary capitalize">{currentPlan}</span></h2>
          <p className="text-sm text-muted">You are currently on the {currentPlan} tier. You have used 4 of your concurrent session limits.</p>
        </div>
        <div className="w-1/3 bg-background border border-border rounded-full h-3 overflow-hidden">
          <div className="bg-primary h-full" style={{ width: currentPlan === 'developer' ? '40%' : '4%' }}></div>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
        {plans.map((plan) => {
          const Icon = plan.icon;
          const isCurrent = currentPlan === plan.id;
          
          return (
            <div 
              key={plan.id}
              className={`relative bg-surface rounded-2xl p-8 flex flex-col ${
                isCurrent 
                  ? 'border-2 border-primary shadow-[0_0_30px_rgba(16,185,129,0.15)]' 
                  : 'border border-border shadow-sm hover:border-border-hover transition-colors'
              }`}
            >
              {isCurrent && (
                <div className="absolute top-0 left-1/2 -translate-x-1/2 -translate-y-1/2 bg-primary text-background text-xs font-bold uppercase tracking-wider py-1 px-3 rounded-full">
                  Current Plan
                </div>
              )}
              
              <Icon size={32} className={isCurrent ? 'text-primary mb-4' : 'text-muted mb-4'} />
              <h3 className="text-2xl font-bold text-foreground mb-2">{plan.name}</h3>
              <div className="flex items-baseline gap-1 mb-4">
                <span className="text-3xl font-extrabold text-foreground">{plan.price}</span>
                {plan.period && <span className="text-muted">/{plan.period}</span>}
              </div>
              <p className="text-muted text-sm mb-8 h-10">{plan.description}</p>
              
              <ul className="space-y-4 mb-8 flex-1">
                {plan.features.map((feat, i) => (
                  <li key={i} className="flex items-start gap-3 text-sm text-foreground">
                    <Check size={18} className="text-primary shrink-0" />
                    {feat}
                  </li>
                ))}
              </ul>
              
              <button
                onClick={() => handleSubscribeClick(plan.id)}
                disabled={isCurrent}
                className={`w-full py-3 rounded-lg font-semibold transition-colors ${
                  isCurrent 
                    ? 'bg-background border border-border text-muted cursor-default'
                    : plan.id === 'enterprise'
                    ? 'bg-surface border border-border text-foreground hover:bg-surface-hover'
                    : 'bg-foreground text-background hover:bg-foreground/90'
                }`}
              >
                {isCurrent ? 'Current Plan' : plan.id === 'enterprise' ? 'Contact Sales' : `Upgrade to ${plan.name}`}
              </button>
            </div>
          );
        })}
      </div>

      {/* Razorpay Mock Modal */}
      <AnimatePresence>
        {showRazorpayMock && selectedPlanToBuy && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4"
          >
            <motion.div
              initial={{ scale: 0.95, y: 20 }}
              animate={{ scale: 1, y: 0 }}
              exit={{ scale: 0.95, y: 20 }}
              className="bg-white rounded-lg shadow-2xl overflow-hidden w-full max-w-[400px]"
            >
              <div className="bg-[#02042b] p-6 text-center">
                <h3 className="text-white font-semibold text-lg">Razorpay Checkout</h3>
                <p className="text-[#a1a1aa] text-sm mt-1">AgentSandbox {selectedPlanToBuy.name} Plan</p>
                <div className="text-3xl text-white font-bold mt-4">{selectedPlanToBuy.price}</div>
              </div>
              
              <div className="p-6">
                <div className="space-y-4">
                  <div className="flex items-center gap-3 p-3 border border-gray-200 rounded-md bg-gray-50 cursor-pointer">
                    <div className="w-8 h-5 bg-gray-200 rounded shrink-0"></div>
                    <span className="text-sm font-medium text-gray-700">Card / EMI</span>
                  </div>
                  <div className="flex items-center gap-3 p-3 border border-gray-200 rounded-md bg-gray-50 cursor-pointer">
                    <div className="w-8 h-5 bg-gray-200 rounded shrink-0 flex items-center justify-center text-[10px] font-bold text-gray-500">UPI</div>
                    <span className="text-sm font-medium text-gray-700">UPI / QR</span>
                  </div>
                  <div className="flex items-center gap-3 p-3 border border-gray-200 rounded-md bg-gray-50 cursor-pointer">
                    <div className="w-8 h-5 bg-gray-200 rounded shrink-0 flex items-center justify-center text-[10px] font-bold text-gray-500">NB</div>
                    <span className="text-sm font-medium text-gray-700">Netbanking</span>
                  </div>
                </div>
                
                <button
                  onClick={simulateRazorpaySuccess}
                  disabled={isProcessing}
                  className="w-full mt-6 bg-[#3399cc] hover:bg-[#2b86b5] text-white font-bold py-3 rounded shadow transition-colors flex items-center justify-center gap-2"
                >
                  {isProcessing ? (
                    <>
                      <Loader2 size={18} className="animate-spin" />
                      Processing...
                    </>
                  ) : (
                    `Pay ${selectedPlanToBuy.price}`
                  )}
                </button>
                
                <button
                  onClick={() => setShowRazorpayMock(false)}
                  disabled={isProcessing}
                  className="w-full mt-3 text-sm text-gray-500 hover:text-gray-800 py-2"
                >
                  Cancel
                </button>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
