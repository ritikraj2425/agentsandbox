import React from 'react';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';

function HomepageHeader() {
  const {siteConfig} = useDocusaurusContext();
  return (
    <header className="heroBanner">
      <div className="heroContent">
        <h1 className="heroTitle">
          Secure, ephemeral sandboxes<br/>for autonomous AI agents.
        </h1>
        <p className="heroSubtitle">
          {siteConfig.tagline}
        </p>
        <div className="heroButtons">
          <Link
            className="button button--primary button--lg"
            to="/docs/intro">
            Read the Documentation
          </Link>
          <Link
            className="button button--secondary button--lg"
            to="https://app.agentsandbox.com">
            Go to Dashboard
          </Link>
        </div>

        <div className="codeWindow">
          <div style={{ display: 'flex', gap: '6px', marginBottom: '16px' }}>
            <div style={{ width: '12px', height: '12px', borderRadius: '50%', backgroundColor: '#ff5f56' }} />
            <div style={{ width: '12px', height: '12px', borderRadius: '50%', backgroundColor: '#ffbd2e' }} />
            <div style={{ width: '12px', height: '12px', borderRadius: '50%', backgroundColor: '#27c93f' }} />
          </div>
          <div>
            <span className="token keyword">curl</span> -X POST https://api.agentsandbox.com/v1/sessions \<br/>
            &nbsp;&nbsp;-H <span className="token string">"Authorization: Bearer sb_live_..."</span> \<br/>
            &nbsp;&nbsp;-H <span className="token string">"Content-Type: application/json"</span> \<br/>
            &nbsp;&nbsp;-d <span className="token string">'&#123;"backend": "firecracker", "ttl": 3600&#125;'</span>
          </div>
        </div>
      </div>
    </header>
  );
}

function HomepageFeatures() {
  return (
    <section className="featureGrid">
      <div className="featureCard">
        <h3>Instant Sandboxes</h3>
        <p>
          Spin up Docker, gVisor, or Firecracker microVMs in milliseconds. Every environment is completely fresh, deterministic, and isolated.
        </p>
      </div>
      <div className="featureCard">
        <h3>Bulletproof Isolation</h3>
        <p>
          Never worry about AI agents dropping a fork bomb or reading your host machine's sensitive files. Run untrusted code with total peace of mind.
        </p>
      </div>
      <div className="featureCard">
        <h3>Declarative Policies</h3>
        <p>
          Define exactly what the agent can do. Block network egress, restrict filesystem reads, and set hard execution timeouts with simple YAML.
        </p>
      </div>
    </section>
  );
}

function HowItWorks() {
  return (
    <section className="howItWorksSection">
      <div className="sectionContent">
        <h2 className="sectionTitle">How It Works</h2>
        <p className="sectionSubtitle">A seamless pipeline from API call to secure execution.</p>
        
        <div className="pipelineContainer">
          <div className="pipelineStep">
            <div className="stepNumber">1</div>
            <h4>API Request</h4>
            <p>Your LLM agent decides to execute a shell command or run python code and sends an HTTP request to our Gateway.</p>
          </div>
          <div className="pipelineConnector"></div>
          <div className="pipelineStep">
            <div className="stepNumber">2</div>
            <h4>Policy Evaluation</h4>
            <p>The Gateway intercepts the request, checks your API key, and evaluates your custom YAML security policies (e.g. denying network egress).</p>
          </div>
          <div className="pipelineConnector"></div>
          <div className="pipelineStep">
            <div className="stepNumber">3</div>
            <h4>MicroVM Execution</h4>
            <p>A Firecracker microVM is booted in ~100ms. The command executes inside the isolated guest OS, and the stdout/stderr is streamed back.</p>
          </div>
          <div className="pipelineConnector"></div>
          <div className="pipelineStep">
            <div className="stepNumber">4</div>
            <h4>Garbage Collection</h4>
            <p>Once the TTL expires, the entire microVM and its filesystem are cryptographically destroyed. No residual state is ever left behind.</p>
          </div>
        </div>
      </div>
    </section>
  );
}

function AboutUs() {
  return (
    <section className="aboutSection">
      <div className="sectionContent">
        <h2 className="sectionTitle">Built for the Autonomous Era</h2>
        <p className="aboutText">
          As Large Language Models become agentic, they need the ability to act upon the world—writing files, browsing the web, and executing code. However, giving an LLM access to a standard environment is a massive security vulnerability. 
        </p>
        <p className="aboutText">
          We built <strong>AgentSandbox</strong> because we realized the bottleneck to autonomous AI wasn't intelligence, it was safety. Our mission is to provide the infrastructure that allows developers to let their agents run wild, without fear of catastrophic system failure.
        </p>
      </div>
    </section>
  );
}

function ProductShowcase() {
  return (
    <section className="showcaseSection">
      <div className="sectionContent">
        <h2 className="sectionTitle">Everything you need.</h2>
        <p className="sectionSubtitle">A powerful CLI for local development, and a beautiful SaaS dashboard for production scaling.</p>
        
        <div className="showcaseGrid">
          <div className="showcaseBox">
            <div className="boxHeader">
              <span className="dot" style={{backgroundColor: '#ff5f56'}}></span>
              <span className="dot" style={{backgroundColor: '#ffbd2e'}}></span>
              <span className="dot" style={{backgroundColor: '#27c93f'}}></span>
              <span className="title">Terminal UI</span>
            </div>
            <div className="boxBody consoleBody">
              <div className="line"><span className="prompt">$</span> agentsandbox run "npm install"</div>
              <div className="line">Spawning isolated environment...</div>
              <div className="line success">[OK] Container started in 42ms</div>
              <div className="line">added 142 packages, and audited 143 packages in 2s</div>
              <div className="line">found 0 vulnerabilities</div>
              <div className="line success">[OK] Session terminated cleanly.</div>
            </div>
          </div>
          
          <div className="showcaseBox dashboardMock">
            <div className="boxHeader dashboardHeader">
              <div className="dashNav">AgentSandbox Dashboard</div>
              <div className="dashUser">user@acme.com</div>
            </div>
            <div className="dashboardBody">
              <div className="sidebar">
                <div className="navItem active">Overview</div>
                <div className="navItem">API Keys</div>
                <div className="navItem">Live Sessions</div>
                <div className="navItem">Billing</div>
              </div>
              <div className="mainDash">
                <h4>Active Sessions</h4>
                <div className="dashCard">
                  <div className="cardRow">
                    <span>sess_89fx2...</span>
                    <span className="badge">Running</span>
                    <span>Docker</span>
                  </div>
                  <div className="cardRow">
                    <span>sess_44zp1...</span>
                    <span className="badge">Running</span>
                    <span>Firecracker</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

export default function Home(): JSX.Element {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title={`${siteConfig.title}`}
      description="AgentSandbox provides secure, ephemeral environments for LLM agents.">
      <HomepageHeader />
      <main>
        <HomepageFeatures />
        <HowItWorks />
        <ProductShowcase />
        <AboutUs />
      </main>
    </Layout>
  );
}
