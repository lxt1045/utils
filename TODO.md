注意：
  需要严格按照 .cursor/rules/ 目录下的规则要求执行。
  在任务完成后，需要总结本次修改，并生成cursor规则，同时将本次工作上下文写到 work.md 文档中，以避免下次修改本项目时重新思考相同内容。

TODO：
1. 这时一个基础库，包含各种实用功能。
2. 帮检查一个各个基础功能是否有BUG或可以改进优化的地方，如果有帮忙修复。
3. 有些 *_test.go 代码由于历史原因已经无法执行，请理解代码上下文并修复。
4. 重点看一下 rpc 目录，该目录是一个双向 rpc 框架。看一下相比已存在的rpc框架，有什么优势和劣势，比较适合什么场景，有什么局限性。
5. rpc/test/socks 目录是 rpc 功能的一个完善的应用场景： 用于http/socks 代理。帮看一下该例子代码有什么问题，client端经常会报这个错误，不知道是否可以修复，并将修复过程和结果写入 README.md 中： 
{"level":"error","time":"2026-04-21T10:21:53.5426366+08:00","caller":"socks/peer_service.go:282","error":{"code":-1,"msg":" n < l, err: write tcp 127.0.0.1:18081->127.0.0.1:52217: wsasend: An established connection was aborted by the software in your host machine.","stack":["(socks/peer_service.go:299) socks.Copy","(socks/peer_client.go:281) socks.(*SocksCli).OutToTCPPeer","(socks/peer_client.go:200) socks.(*SocksCli).connectHttp","(runtime/asm_amd64.s:1693) runtime.goexit"]},"logid":1907757491794477056,"message":"Copy defer"}
