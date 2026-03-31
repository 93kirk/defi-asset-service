#!/usr/bin/env python3
"""
DeFi资产展示服务 - Redis队列生产者测试脚本
用于测试生产者API的功能
"""

import requests
import json
import time
import random
from typing import List, Dict, Any
from datetime import datetime

class QueueProducerTester:
    def __init__(self, base_url: str = "http://localhost:8081"):
        self.base_url = base_url
        self.session = requests.Session()
        
    def health_check(self) -> bool:
        """检查服务健康状态"""
        try:
            response = self.session.get(f"{self.base_url}/health")
            if response.status_code == 200:
                data = response.json()
                print(f"✅ 健康检查通过: {data.get('status')}")
                return True
            else:
                print(f"❌ 健康检查失败: {response.status_code}")
                return False
        except Exception as e:
            print(f"❌ 健康检查异常: {e}")
            return False
    
    def get_stats(self) -> Dict[str, Any]:
        """获取队列统计信息"""
        try:
            response = self.session.get(f"{self.base_url}/stats")
            if response.status_code == 200:
                return response.json()
            else:
                print(f"❌ 获取统计失败: {response.status_code}")
                return {}
        except Exception as e:
            print(f"❌ 获取统计异常: {e}")
            return {}
    
    def publish_single_message(self, user_address: str, protocol_id: str, position_data: Dict[str, Any]) -> str:
        """发布单个消息"""
        payload = {
            "user_address": user_address,
            "protocol_id": protocol_id,
            "position_data": position_data
        }
        
        try:
            response = self.session.post(
                f"{self.base_url}/publish",
                json=payload,
                headers={"Content-Type": "application/json"}
            )
            
            if response.status_code == 201:
                data = response.json()
                message_id = data.get("message_id")
                print(f"✅ 消息发布成功: {message_id}")
                return message_id
            else:
                print(f"❌ 消息发布失败: {response.status_code} - {response.text}")
                return ""
        except Exception as e:
            print(f"❌ 消息发布异常: {e}")
            return ""
    
    def publish_batch_messages(self, messages: List[Dict[str, Any]]) -> List[str]:
        """批量发布消息"""
        try:
            response = self.session.post(
                f"{self.base_url}/publish/batch",
                json=messages,
                headers={"Content-Type": "application/json"}
            )
            
            if response.status_code == 201:
                data = response.json()
                message_ids = data.get("message_ids", [])
                print(f"✅ 批量发布成功: {len(message_ids)} 条消息")
                return message_ids
            else:
                print(f"❌ 批量发布失败: {response.status_code} - {response.text}")
                return []
        except Exception as e:
            print(f"❌ 批量发布异常: {e}")
            return []
    
    def get_metrics(self) -> Dict[str, Any]:
        """获取监控指标"""
        try:
            response = self.session.get(f"{self.base_url}/metrics")
            if response.status_code == 200:
                return response.json()
            else:
                print(f"❌ 获取指标失败: {response.status_code}")
                return {}
        except Exception as e:
            print(f"❌ 获取指标异常: {e}")
            return {}

def generate_test_user_address() -> str:
    """生成测试用户地址"""
    # 生成随机的以太坊地址格式
    hex_chars = "0123456789abcdef"
    random_hex = ''.join(random.choice(hex_chars) for _ in range(40))
    return f"0x{random_hex}"

def generate_test_position_data() -> Dict[str, Any]:
    """生成测试仓位数据"""
    tokens = [
        {"address": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", "symbol": "USDC"},
        {"address": "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2", "symbol": "WETH"},
        {"address": "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599", "symbol": "WBTC"},
        {"address": "0x6B175474E89094C44Da98b954EedeAC495271d0F", "symbol": "DAI"},
        {"address": "0xdAC17F958D2ee523a2206206994597C13D831ec7", "symbol": "USDT"},
    ]
    
    token = random.choice(tokens)
    amount = random.uniform(100, 10000)
    amount_usd = amount * random.uniform(0.9, 1.1)
    apy = random.uniform(1.0, 10.0)
    
    return {
        "token_address": token["address"],
        "token_symbol": token["symbol"],
        "amount": f"{amount:.2f}",
        "amount_usd": f"{amount_usd:.2f}",
        "apy": f"{apy:.2f}",
        "risk_level": random.randint(1, 5),
        "metadata": {
            "test": True,
            "timestamp": datetime.now().isoformat()
        }
    }

def generate_test_protocol() -> str:
    """生成测试协议ID"""
    protocols = ["aave", "compound", "uniswap", "curve", "maker", "lido", "yearn", "balancer"]
    return random.choice(protocols)

def run_comprehensive_test():
    """运行综合测试"""
    print("=" * 60)
    print("DeFi资产展示服务 - Redis队列生产者测试")
    print("=" * 60)
    
    tester = QueueProducerTester()
    
    # 1. 健康检查
    print("\n1. 健康检查")
    if not tester.health_check():
        print("❌ 服务不可用，测试终止")
        return
    
    # 2. 获取初始统计
    print("\n2. 获取初始统计")
    initial_stats = tester.get_stats()
    if initial_stats:
        print(f"   队列长度: {initial_stats.get('message_count', 0)}")
        print(f"   消费者数量: {initial_stats.get('consumer_count', 0)}")
    
    # 3. 发布单个消息
    print("\n3. 发布单个消息")
    user_address = generate_test_user_address()
    protocol_id = generate_test_protocol()
    position_data = generate_test_position_data()
    
    message_id = tester.publish_single_message(user_address, protocol_id, position_data)
    
    # 4. 批量发布消息
    print("\n4. 批量发布消息")
    batch_messages = []
    for i in range(5):
        batch_messages.append({
            "user_address": generate_test_user_address(),
            "protocol_id": generate_test_protocol(),
            "position_data": generate_test_position_data()
        })
    
    message_ids = tester.publish_batch_messages(batch_messages)
    
    # 5. 获取最终统计
    print("\n5. 获取最终统计")
    time.sleep(2)  # 等待消息处理
    final_stats = tester.get_stats()
    if final_stats:
        print(f"   队列长度: {final_stats.get('message_count', 0)}")
        print(f"   消费者数量: {final_stats.get('consumer_count', 0)}")
        
        # 计算新增消息
        initial_count = initial_stats.get('message_count', 0)
        final_count = final_stats.get('message_count', 0)
        new_messages = final_count - initial_count
        print(f"   新增消息: {new_messages} (预期: {len(message_ids) + (1 if message_id else 0)})")
    
    # 6. 获取监控指标
    print("\n6. 获取监控指标")
    metrics = tester.get_metrics()
    if metrics:
        queue_stats = metrics.get('queue_stats', {})
        print(f"   Redis连接: {'正常' if metrics.get('redis_info', {}).get('connected') else '异常'}")
        print(f"   队列统计: {queue_stats}")
    
    print("\n" + "=" * 60)
    print("测试完成!")
    print("=" * 60)

def run_performance_test(num_messages: int = 100):
    """运行性能测试"""
    print(f"\n性能测试: 发布 {num_messages} 条消息")
    
    tester = QueueProducerTester()
    
    if not tester.health_check():
        return
    
    # 准备测试数据
    test_messages = []
    for i in range(num_messages):
        test_messages.append({
            "user_address": generate_test_user_address(),
            "protocol_id": generate_test_protocol(),
            "position_data": generate_test_position_data()
        })
    
    # 分批发布（每批10条）
    batch_size = 10
    total_batches = (num_messages + batch_size - 1) // batch_size
    
    start_time = time.time()
    successful_messages = 0
    
    for batch_num in range(total_batches):
        batch_start = batch_num * batch_size
        batch_end = min(batch_start + batch_size, num_messages)
        batch = test_messages[batch_start:batch_end]
        
        message_ids = tester.publish_batch_messages(batch)
        successful_messages += len(message_ids)
        
        # 显示进度
        progress = (batch_num + 1) / total_batches * 100
        print(f"   进度: {progress:.1f}% ({batch_num + 1}/{total_batches} 批)")
    
    end_time = time.time()
    elapsed_time = end_time - start_time
    
    print(f"\n性能测试结果:")
    print(f"   总消息数: {num_messages}")
    print(f"   成功消息: {successful_messages}")
    print(f"   成功率: {(successful_messages / num_messages * 100):.1f}%")
    print(f"   总耗时: {elapsed_time:.2f} 秒")
    print(f"   平均速度: {(num_messages / elapsed_time):.1f} 消息/秒")
    print(f"   批处理速度: {(successful_messages / elapsed_time):.1f} 消息/秒")

def main():
    """主函数"""
    import argparse
    
    parser = argparse.ArgumentParser(description="DeFi队列生产者测试工具")
    parser.add_argument("--test", choices=["comprehensive", "performance", "health"], 
                       default="comprehensive", help="测试类型")
    parser.add_argument("--url", default="http://localhost:8081", 
                       help="生产者API地址")
    parser.add_argument("--count", type=int, default=100,
                       help="性能测试消息数量")
    
    args = parser.parse_args()
    
    if args.test == "comprehensive":
        run_comprehensive_test()
    elif args.test == "performance":
        run_performance_test(args.count)
    elif args.test == "health":
        tester = QueueProducerTester(args.url)
        tester.health_check()

if __name__ == "__main__":
    main()