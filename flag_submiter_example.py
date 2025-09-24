#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Flag提交器示例代码
这个脚本展示了如何接收flags参数并返回提交结果

使用方法：
1. 在Action的Config字段中设置：
   {"type": "flag_submiter", "num": 4}
   
2. 在Action的Code字段中设置base64编码的Python代码

3. 系统会自动传入flags参数，格式为JSON数组字符串
"""

import sys
import json

def main():
    # 获取传入的flags参数
    if len(sys.argv) > 1:
        try:
            # 解析JSON格式的flags
            flags = json.loads(sys.argv[1])
            print(f"接收到 {len(flags)} 个flags: {flags}")
        except json.JSONDecodeError as e:
            print(f"解析flags参数失败: {e}")
            return
    else:
        print("没有接收到flags参数")
        return
    
    # 模拟flag提交过程
    results = []
    for flag in flags:
        # 这里应该是实际的flag提交逻辑
        # 例如：发送HTTP请求到flag提交服务器
        
        # 模拟不同的提交结果
        if "test" in flag.lower():
            status = "SUCCESS"
            msg = "Flag提交成功"
        elif "invalid" in flag.lower():
            status = "INVALID"
            msg = "Flag格式无效"
        else:
            status = "DUPLICATE"
            msg = "Flag已存在"
        
        results.append({
            "flag": flag,
            "status": status,
            "msg": msg
        })
    
    # 输出结果，必须是JSON格式
    print(json.dumps(results, ensure_ascii=False))

if __name__ == "__main__":
    main()
