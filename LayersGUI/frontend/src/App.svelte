<script lang="ts">
  import { RunLayerTests, GetReportPath } from '../wailsjs/go/main/App'

  let selectedLayers: number[] = []
  let testResults: any[] = []
  let loading = false
  let reportPath = ''
  let networkDetails: any = null
  let vulnerabilityFindings: any[] = []
  let progress = 0
  let currentLayer = ''

  // Array of OSI layers
  const layers = [
    { id: 1, name: 'Physical Layer', description: 'Hardware transmission' },
    { id: 2, name: 'Data Link Layer', description: 'Framing and error control' },
    { id: 3, name: 'Network Layer', description: 'Routing and addressing' },
    { id: 4, name: 'Transport Layer', description: 'End-to-end connections' },
    { id: 5, name: 'Session Layer', description: 'Session management' },
    { id: 6, name: 'Presentation Layer', description: 'Data formatting' },
    { id: 7, name: 'Application Layer', description: 'User interface' }
  ]

  async function runTests() {
    if (selectedLayers.length === 0) return

    loading = true
    testResults = [] // Clear previous results
    progress = 0
    
    try {
      // Run tests for each layer independently
      for (const layerId of selectedLayers) {
        const layer = layers.find(l => l.id === layerId)
        currentLayer = `Layer ${layerId}: ${layer?.name}`
        progress = (selectedLayers.indexOf(layerId) / selectedLayers.length) * 100
        
        // Run single layer test
        const result = await RunLayerTests([layerId])
        if (result && result.length > 0) {
          // Process the result
          const processedResult = result[0]
          if (processedResult.message?.toLowerCase().includes('cancelled')) {
            processedResult.status = 'Failed'
            processedResult.message = 'Test failed - Unable to complete layer analysis'
            processedResult.details = processedResult.message
          }
          testResults.push(processedResult)
        }
      }
      
      progress = 100
      reportPath = await GetReportPath()
      processNetworkDetails()
      analyzeVulnerabilities()
    } catch (error) {
      console.error('Test execution failed:', error)
      testResults = selectedLayers.map(layer => ({
        layer,
        status: 'Failed',
        message: 'Test execution error',
        error: error.toString()
      }))
    } finally {
      loading = false
      progress = 0
      currentLayer = ''
    }
  }

  function processNetworkDetails() {
    // Extract network details from test results
    const layer1Results = testResults.find(r => r.layer === 1)
    const layer2Results = testResults.find(r => r.layer === 2)
    
    if (layer1Results?.sub_results && layer2Results?.sub_results) {
      networkDetails = {
        interfaces: layer1Results.sub_results.map(iface => ({
          name: iface.name.replace('Interface ', '').replace(' Connection', ''),
          status: iface.status,
          addresses: iface.diagnostics?.addresses || [],
          type: iface.diagnostics?.type || 'Unknown',
          isVPN: iface.diagnostics?.is_vpn || false,
          metrics: {
            txBytes: iface.diagnostics?.tx_bytes || 0,
            rxBytes: iface.diagnostics?.rx_bytes || 0,
            signalStrength: iface.diagnostics?.signal_strength,
            linkQuality: iface.diagnostics?.link_quality
          }
        }))
      }
    }
  }

  function analyzeVulnerabilities() {
    vulnerabilityFindings = []
    
    // Analyze open ports and potential vulnerabilities
    testResults.forEach(result => {
      if (result.diagnostics?.open_ports) {
        result.diagnostics.open_ports.forEach(port => {
          if (isVulnerablePort(port)) {
            vulnerabilityFindings.push({
              severity: 'High',
              type: 'Open Vulnerable Port',
              details: `Port ${port} is open and potentially vulnerable`,
              recommendation: getPortSecurityRecommendation(port)
            })
          }
        })
      }
    })
  }

  function isVulnerablePort(port: number): boolean {
    const vulnerablePorts = [21, 23, 25, 53, 137, 139, 445, 3389] // Example vulnerable ports
    return vulnerablePorts.includes(port)
  }

  function getPortSecurityRecommendation(port: number): string {
    const recommendations = {
      21: 'Consider using SFTP (port 22) instead of FTP',
      23: 'Disable Telnet and use SSH instead',
      25: 'Secure SMTP with TLS or use submission port 587',
      53: 'Ensure DNS is properly configured and consider DNS-over-TLS',
      137: 'Disable NetBIOS if not required',
      139: 'Disable NetBIOS if not required',
      445: 'Ensure SMB is properly configured and updated',
      3389: 'Use RDP over VPN and enable Network Level Authentication'
    }
    return recommendations[port] || 'Consider closing this port if not required'
  }

  function toggleLayer(layerId: number) {
    const index = selectedLayers.indexOf(layerId)
    if (index === -1) {
      selectedLayers = [...selectedLayers, layerId]
    } else {
      selectedLayers = selectedLayers.filter(id => id !== layerId)
    }
  }

  function selectAllLayers() {
    selectedLayers = layers.map(layer => layer.id)
  }

  function clearSelection() {
    selectedLayers = []
    testResults = []
    reportPath = ''
    networkDetails = null
    vulnerabilityFindings = []
  }

  function formatBytes(bytes: number): string {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`
  }
</script>

{#if loading}
	<div class="loading-container">
		<div class="loading-content">
			<div class="progress-bar">
				<div class="progress-fill" style="width: {progress}%"></div>
			</div>
			<div class="loading-text">
				<p class="current-layer">{currentLayer}</p>
				<p class="progress-text">Progress: {Math.round(progress)}%</p>
			</div>
		</div>
	</div>
{:else}
	<main class="container">
		<header>
			<h1>Layers</h1>
			<p class="copyright">© Aaron Stovall</p>
		</header>

		<div class="controls">
			<button on:click={selectAllLayers}>Select All Layers</button>
			<button on:click={clearSelection}>Clear Selection</button>
			<button on:click={runTests} disabled={selectedLayers.length === 0 || loading}>
				{loading ? 'Running Tests...' : 'Run Tests'}
			</button>
		</div>

		<div class="layers">
			{#each layers as layer}
				<div 
					class="layer" 
					class:selected={selectedLayers.includes(layer.id)}
					on:click={() => toggleLayer(layer.id)}
					on:keydown={(e) => e.key === 'Enter' && toggleLayer(layer.id)}
					tabindex="0"
					role="button"
				>
					<h3>Layer {layer.id}: {layer.name}</h3>
					<p>{layer.description}</p>
				</div>
			{/each}
		</div>

		{#if testResults && testResults.length > 0}
			<div class="results">
				<h2>Test Results</h2>
				{#each testResults as result}
					{#if result.layer === 8}
						<div class="security-findings">
							<h2>Security Assessment</h2>
							
							<!-- Network Details -->
							{#if result.diagnostics?.network_info}
								<div class="network-details">
									<h3>Network Adapter Details</h3>
									<div class="adapters">
										{#each result.diagnostics.network_info as netInterface}
											<div class="adapter" class:adapter-passed={netInterface.status === 'UP'} class:adapter-failed={netInterface.status === 'DOWN'}>
												<h3>{netInterface.interfaceName}</h3>
												<div class="adapter-details">
													<p class="status {netInterface.status.toLowerCase()}">{netInterface.status}</p>
													{#if netInterface.isPrimary}
														<p class="badge primary">Primary</p>
													{/if}
													{#if netInterface.isVPN}
														<p class="badge vpn">VPN</p>
													{/if}
													{#if netInterface.ipv4Address?.length > 0 || netInterface.ipv6Address?.length > 0}
														<div class="ip-addresses">
															<h4>IP Addresses:</h4>
															<ul>
																{#each netInterface.ipv4Address || [] as ip}
																	<li>IPv4: {ip}</li>
																{/each}
																{#each netInterface.ipv6Address || [] as ip}
																	<li>IPv6: {ip}</li>
																{/each}
															</ul>
														</div>
													{/if}
												</div>
											</div>
										{/each}
									</div>
								</div>
							{/if}

							<!-- Open Ports -->
							{#if result.diagnostics?.open_ports}
								<div class="ports">
									<h3>Open Ports</h3>
									<div class="port-list">
										{#each result.diagnostics.open_ports as port}
											<div class="port" class:vulnerable={port.isVulnerable}>
												<span class="port-number">:{port.port}</span>
												<span class="port-service">{port.service}</span>
												<span class="port-protocol">{port.protocol}</span>
												{#if port.isVulnerable}
													<span class="vulnerability-warning">Potentially Vulnerable</span>
												{/if}
											</div>
										{/each}
									</div>
								</div>
							{/if}

							<!-- Vulnerabilities -->
							{#if result.diagnostics?.vulnerabilities}
								<div class="vulnerabilities">
									<h3>Security Findings</h3>
									<ul class="vulnerability-list">
										{#each result.diagnostics.vulnerabilities as vuln}
											<li class="vulnerability-item">{vuln}</li>
										{/each}
									</ul>
								</div>
							{/if}
						</div>
					{:else}
						<div class="result" 
							class:passed={result.status.toUpperCase() === 'PASSED'} 
							class:failed={result.status.toUpperCase() === 'FAILED'} 
							class:warning={result.status.toUpperCase() === 'WARNING'} 
							class:skipped={result.status.toUpperCase() === 'SKIPPED'}>
							<div class="result-header">
								<h3 class="layer-title">Layer {result.layer}</h3>
								<p class="status {result.status.toLowerCase()}">{result.status}</p>
							</div>
							{#if result.error}
								<p class="error-message">{result.error}</p>
							{:else if result.details}
								<p class="details-message">{result.details}</p>
								<p class="message">{result.message}</p>
							{:else}
								<p class="message">{result.message || 'No additional information available'}</p>
							{/if}
							{#if result.sub_results}
								<div class="sub-results">
									{#each result.sub_results as subResult}
										<div class="sub-result" class:passed={subResult.status === 'Passed'} class:failed={subResult.status === 'Failed'} class:warning={subResult.status === 'Warning'}>
											<h4>{subResult.name}</h4>
											<p class="status">{subResult.status}</p>
											<p class="message">{subResult.message}</p>
										</div>
									{/each}
								</div>
							{/if}
						</div>
					{/if}
				{/each}
			</div>
		{/if}

		{#if reportPath}
			<p class="report-path">Report saved to: {reportPath}</p>
		{/if}
	</main>
{/if}

<style>
	:global(body) {
		margin: 0;
		padding: 0;
		background-color: #1a1a1a;
		color: #ffffff;
	}

	.container {
		max-width: 1200px;
		margin: 0 auto;
		padding: 2rem;
		text-align: center;
	}

	header {
		margin-bottom: 3rem;
	}

	h1 {
		color: #00ff00;
		font-size: 3rem;
		margin: 0;
		text-shadow: 0 0 20px rgba(0, 255, 0, 0.5);
		font-weight: bold;
	}

	.copyright {
		color: #888;
		margin: 0.5rem 0 0 0;
		font-size: 0.9rem;
	}

	.controls {
		display: flex;
		gap: 1rem;
		margin-bottom: 3rem;
		justify-content: center;
	}

	button {
		background: transparent;
		color: #00ff00;
		border: 2px solid #00ff00;
		padding: 0.75rem 1.5rem;
		cursor: pointer;
		transition: all 0.3s ease;
		font-size: 1rem;
		text-transform: uppercase;
		letter-spacing: 1px;
		box-shadow: 0 0 10px rgba(0, 255, 0, 0.2);
	}

	button:hover {
		background: #00ff00;
		color: #1a1a1a;
		box-shadow: 0 0 20px rgba(0, 255, 0, 0.4);
	}

	button:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.layers {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
		gap: 1.5rem;
		margin-bottom: 2rem;
	}

	.layer {
		background: rgba(0, 0, 0, 0.3);
		border: 1px solid #333;
		padding: 1.5rem;
		cursor: pointer;
		transition: all 0.3s ease;
		border-radius: 4px;
		text-align: left;
	}

	.layer:hover {
		border-color: #00ff00;
		background: rgba(0, 255, 0, 0.05);
	}

	.layer.selected {
		background: rgba(0, 255, 0, 0.15);
		border: 2px solid #00ff00;
	}

	.layer h3 {
		color: #888;
		margin: 0 0 0.5rem 0;
		font-size: 1.2rem;
		transition: color 0.3s ease;
	}

	.layer.selected h3 {
		color: #00ff00;
	}

	.layer p {
		color: #888;
		margin: 0;
		font-size: 0.9rem;
	}

	.loading-container {
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		background: rgba(0, 0, 0, 0.9);
		display: flex;
		justify-content: center;
		align-items: center;
		z-index: 1000;
	}

	.loading-content {
		width: 80%;
		max-width: 600px;
		text-align: center;
	}

	.progress-bar {
		width: 100%;
		height: 10px;
		background: rgba(0, 255, 0, 0.1);
		border: 1px solid #00ff00;
		border-radius: 5px;
		overflow: hidden;
		margin-bottom: 1rem;
		box-shadow: 0 0 10px rgba(0, 255, 0, 0.2);
	}

	.progress-fill {
		height: 100%;
		background: #00ff00;
		transition: width 0.3s ease;
		box-shadow: 0 0 20px rgba(0, 255, 0, 0.4);
	}

	.loading-text {
		color: #00ff00;
		text-shadow: 0 0 10px rgba(0, 255, 0, 0.3);
	}

	.current-layer {
		font-size: 1.5rem;
		margin-bottom: 0.5rem;
	}

	.progress-text {
		font-size: 1rem;
		opacity: 0.8;
	}

	.results {
		margin-top: 2rem;
	}

	.result {
		background: #2a2a2a;
		padding: 1.5rem;
		margin-bottom: 1.5rem;
		border-radius: 4px;
		border: 1px solid #333;
		text-align: left;
	}

	.result-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 1rem;
	}

	.layer-title {
		color: #00ff00;
		margin: 0;
		font-size: 1.5rem;
	}

	.details-message {
		color: #888;
		margin: 0.5rem 0;
		font-size: 0.95rem;
		font-style: italic;
		padding: 0.5rem;
		border-radius: 3px;
		background: rgba(255, 255, 255, 0.05);
	}

	.error-message {
		color: #ff0000;
		margin: 0.5rem 0;
		font-size: 0.95rem;
		background: rgba(255, 0, 0, 0.1);
		padding: 0.5rem;
		border-radius: 3px;
		border: 1px solid rgba(255, 0, 0, 0.2);
	}

	.status {
		font-weight: bold;
		padding: 0.5rem 1rem;
		border-radius: 3px;
		display: inline-block;
		text-transform: uppercase;
		letter-spacing: 1px;
		font-size: 1rem;
		min-width: 100px;
		text-align: center;
	}

	.status.passed {
		background: rgba(0, 255, 0, 0.15);
		color: #00ff00;
		border: 1px solid #00ff00;
		box-shadow: 0 0 10px rgba(0, 255, 0, 0.2);
	}

	.status.failed {
		background: rgba(255, 0, 0, 0.15);
		color: #ff0000;
		border: 1px solid #ff0000;
		box-shadow: 0 0 10px rgba(255, 0, 0, 0.2);
	}

	.status.warning {
		background: rgba(255, 255, 0, 0.15);
		color: #ffff00;
		border: 1px solid #ffff00;
		box-shadow: 0 0 10px rgba(255, 255, 0, 0.2);
	}

	.status.skipped {
		background: rgba(128, 128, 128, 0.15);
		color: #888888;
		border: 1px solid #888888;
		box-shadow: 0 0 10px rgba(128, 128, 128, 0.2);
	}

	.message {
		margin: 0.5rem 0;
		color: #888;
		font-size: 0.95rem;
	}

	.sub-results {
		margin-top: 1rem;
		padding-left: 1rem;
	}

	.sub-result {
		margin: 0.5rem 0;
		padding: 1rem;
		border-radius: 4px;
		background: rgba(0, 0, 0, 0.2);
		border: 1px solid #333;
	}

	.sub-result.passed {
		border-left: 4px solid #00ff00;
	}

	.sub-result.failed {
		border-left: 4px solid #ff0000;
	}

	.sub-result.warning {
		border-left: 4px solid #ffff00;
	}

	.sub-result h4 {
		margin: 0 0 0.5rem 0;
		color: #00ff00;
	}

	.security-findings {
		background: #2a2a2a;
		padding: 1.5rem;
		margin-bottom: 2rem;
		border-radius: 4px;
	}

	.network-details {
		margin-bottom: 2rem;
	}

	.adapters {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
		gap: 1rem;
	}

	.adapter {
		background: #1e1e1e;
		padding: 1rem;
		border-radius: 4px;
		border: 1px solid #333;
		position: relative;
		margin-bottom: 1rem;
	}

	.adapter::before {
		content: '';
		position: absolute;
		top: 0;
		right: 0;
		width: 10px;
		height: 10px;
		border-radius: 50%;
		margin: 1rem;
	}

	.adapter-passed::before {
		background: #00ff00;
		box-shadow: 0 0 10px #00ff00;
	}

	.adapter-failed::before {
		background: #ff0000;
		box-shadow: 0 0 10px #ff0000;
	}

	.adapter h3 {
		color: #00ff00;
		margin: 0;
		padding-right: 2rem;
	}

	.adapter-details {
		margin-top: 1rem;
	}

	.badge {
		display: inline-block;
		padding: 0.25rem 0.5rem;
		border-radius: 3px;
		font-size: 0.875rem;
		margin-left: 0.5rem;
	}

	.badge.primary {
		background: rgba(0, 128, 255, 0.1);
		color: #0088ff;
	}

	.badge.vpn {
		background: rgba(128, 0, 255, 0.1);
		color: #8800ff;
	}

	.ip-addresses {
		margin: 1rem 0;
	}

	.ip-addresses ul {
		list-style: none;
		padding: 0;
		margin: 0;
	}

	.ip-addresses li {
		padding: 0.25rem 0;
		color: #00ff00;
	}

	.ports {
		margin-top: 2rem;
	}

	.port-list {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
		gap: 1rem;
	}

	.port {
		background: #1e1e1e;
		padding: 1rem;
		border-radius: 4px;
		border: 1px solid #333;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.port.vulnerable {
		border-left: 4px solid #ff0000;
	}

	.port-number {
		font-size: 1.25rem;
		font-weight: bold;
		color: #00ff00;
	}

	.port-service {
		color: #888;
	}

	.port-protocol {
		color: #666;
		font-size: 0.875rem;
	}

	.vulnerability-warning {
		color: #ff0000;
		font-weight: bold;
	}

	.vulnerabilities {
		margin-top: 2rem;
	}

	.vulnerability-list {
		list-style: none;
		padding: 0;
		margin: 0;
	}

	.vulnerability-item {
		background: rgba(255, 0, 0, 0.1);
		border-left: 4px solid #ff0000;
		padding: 1rem;
		margin-bottom: 0.5rem;
		border-radius: 4px;
		color: #ff0000;
	}

	.report-path {
		margin-top: 1rem;
		padding: 1rem;
		background: #2a2a2a;
		color: #00ff00;
		border-radius: 4px;
		border-left: 4px solid #00ff00;
	}
</style>
