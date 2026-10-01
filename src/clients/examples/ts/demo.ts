import { Bus } from '../../typescript/src/index.ts';
const bus=Bus.connect();
try {
  const result=await bus.call('#stats-ts@localhost',JSON.stringify({path:process.argv[2]}),{topic:'validate-json',timeout:30_000});
  if(result.kind!=='answer')throw new Error(`validator ended with ${result.kind}`);
  const report=JSON.parse(result.message.body);
  console.log(JSON.stringify(report,null,2));
  if(report.worker!=='ts' || report.valid!==true)throw new Error('JSON validation demo failed');
} finally { await bus.close(); }
