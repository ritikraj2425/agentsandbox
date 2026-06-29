import LoginForm from '@/components/LoginForm';

export const metadata = {
  title: 'Login - AgentSandbox',
};

export default function LoginPage() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4 relative overflow-hidden">
      {/* Background decorations */}
      <div className="absolute top-0 left-0 w-full h-full overflow-hidden pointer-events-none">
        <div className="absolute -top-[20%] -left-[10%] w-[50%] h-[50%] rounded-full bg-primary/5 blur-[120px]" />
        <div className="absolute top-[60%] right-[10%] w-[40%] h-[40%] rounded-full bg-purple-500/5 blur-[100px]" />
      </div>

      <div className="w-full max-w-md relative z-10">
        <div className="text-center mb-8 text-white">
          <h1 className="text-3xl font-bold mb-2">AgentSandbox</h1>
          <p className="text-muted">Connect to your secure environment</p>
        </div>

        <LoginForm />

        <div className="text-center mt-6">
          <p className="text-xs text-muted">AgentSandbox v0.6.0</p>
        </div>
      </div>
    </div>
  );
}
